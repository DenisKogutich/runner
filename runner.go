package runner

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Runner receives lists of jobs to be started or stopped.
// It preserves the order of incoming actions and is able to stop work that is in the process of starting.
type Runner struct {
	cfg      *Config
	wg       *sync.WaitGroup
	guard    *sync.RWMutex
	limiter  *ConcurrentLimiter
	running  map[int64]Job
	stopper  map[int64]func()
	stop     chan struct{}
	commands *CommandsChannel
	// expected count of running jobs (running + scheduled), used for logging
	expectedCount int32
	// external changes
	add    <-chan []Job
	delete <-chan []Job
}

// Config keeps Runner's settings.
type Config struct {
	MaxParallelStarts uint
	RetryDelay        time.Duration
}

func NewRunner(cfg *Config, add <-chan []Job, delete <-chan []Job) *Runner {
	return &Runner{
		cfg:     cfg,
		wg:      new(sync.WaitGroup),
		guard:   new(sync.RWMutex),
		limiter: NewConcurrentLimiter(cfg.MaxParallelStarts),
		running: make(map[int64]Job),
		stopper: make(map[int64]func()),
		add:     add,
		delete:  delete,
	}
}

func (r *Runner) Start() {
	r.stop = make(chan struct{})
	r.commands = NewCommandsChannel()

	r.wg.Add(2)
	go r.eventLoop()
	go r.cmdLoop()
}

func (r *Runner) Stop() {
	close(r.stop)
	r.wg.Wait()

	r.commands.Close()
	r.stop = nil

	r.guard.Lock()
	defer r.guard.Unlock()

	for _, stopper := range r.stopper {
		stopper()
	}
}

func (r *Runner) Job(id int64) (Job, bool) {
	r.guard.RLock()
	defer r.guard.RUnlock()

	j, exist := r.running[id]
	return j, exist
}

func (r *Runner) eventLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stop:
			return
		case add := <-r.add:
			atomic.AddInt32(&r.expectedCount, int32(len(add)))
			for _, job := range add {
				r.commands.Put(NewStartCommand(job))
			}
		case del := <-r.delete:
			atomic.AddInt32(&r.expectedCount, -int32(len(del)))
			for _, job := range del {
				r.commands.Put(NewStopCommand(job))
			}
		}
	}
}

func (r *Runner) cmdLoop() {
	defer r.wg.Done()
	cmdChan := r.commands.Chan()

	for {
		select {
		case <-r.stop:
			return
		case cmd := <-cmdChan:
			job := cmd.Job()

			// stop
			if cmd.Type() == Stop {
				r.stopJob(job)
				continue
			}

			// start
			r.guard.RLock()
			_, skip := r.stopper[job.ID()]
			r.guard.RUnlock()

			// job already running or scheduled for running
			if skip {
				continue
			}

			ctx, cancel := context.WithCancel(context.Background())
			r.guard.Lock()
			r.stopper[job.ID()] = func() {
				cancel()
				delete(r.stopper, job.ID())
			}
			r.guard.Unlock()

			r.startJob(ctx, job)
		}
	}
}

func (r *Runner) startJob(ctx context.Context, job Job) {
	if err := r.limiter.Acquire(ctx); err != nil {
		// job start canceled
		log.Printf("[INF] %q start canceled\n", job)
		return
	}

	go func() {
		defer r.limiter.Release()

		if err := job.Start(ctx); err == nil {
			r.guard.Lock()
			defer r.guard.Unlock()

			// additional check for ctx cancel in case of job not handle ctx correctly
			select {
			case <-ctx.Done():
				job.Stop()
				log.Printf("[INF] %q stopped\n", job)
				return
			default:
			}

			log.Printf("[INF] %d/%d, %q started \n", len(r.running)+1, atomic.LoadInt32(&r.expectedCount), job)
			// cancel ctx to release context resources
			r.stopper[job.ID()]()
			r.running[job.ID()] = job
			r.stopper[job.ID()] = func() {
				job.Stop()
				log.Printf("[INF] %q stopped\n", job)
				delete(r.running, job.ID())
				delete(r.stopper, job.ID())
			}

			return
		} else {
			log.Printf("[ERR] %q start failed\n", job)
			// schedule start job retry
			go func() {
				select {
				case <-ctx.Done():
					// job start canceled
					log.Printf("[INF] %q start canceled\n", job)
					return
				case <-time.After(r.cfg.RetryDelay):
					r.startJob(ctx, job)
				}
			}()
		}
	}()
}

func (r *Runner) stopJob(job Job) {
	r.guard.Lock()
	defer r.guard.Unlock()

	if stopper, exist := r.stopper[job.ID()]; exist {
		stopper()
	}
}
