package runner

import (
	"context"
	"sync"
	"time"
)

type Runner struct {
	cfg     *Config
	wg      *sync.WaitGroup
	guard   *sync.RWMutex
	limiter *ConcurrentLimiter
	running map[string]Job
	stopper map[string]func()
	stopC   chan struct{}

	// external changes
	add    <-chan []Job
	delete <-chan []Job
}

func NewRunner(cfg *Config, add <-chan []Job, delete <-chan []Job) *Runner {
	return &Runner{
		cfg:     cfg,
		wg:      new(sync.WaitGroup),
		guard:   new(sync.RWMutex),
		limiter: NewConcurrentLimiter(cfg.MaxParallelStarts),
		running: make(map[string]Job),
		stopper: make(map[string]func()),
		add:     add,
		delete:  delete,
	}
}

func (r *Runner) Start() {
	r.stopC = make(chan struct{})
	r.wg.Add(1)
	go r.loop()
}

func (r *Runner) Stop() {
	close(r.stopC)
	r.wg.Wait()
	r.stopC = nil

	r.guard.Lock()
	defer r.guard.Unlock()

	for _, stopper := range r.stopper {
		stopper()
	}
}

func (r *Runner) loop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopC:
			return
		case add := <-r.add:
			for _, j := range add {
				name := j.Name

				r.guard.RLock()
				_, scheduled := r.stopper[name]
				r.guard.RUnlock()

				// job already scheduled for running
				if scheduled {
					continue
				}

				ctx, cancel := context.WithCancel(context.Background())

				r.guard.Lock()
				r.stopper[name] = func() {
					cancel()
					delete(r.stopper, name)
				}
				r.guard.Unlock()

				r.addJob(ctx, j)
			}
		case del := <-r.delete:
			r.deleteJobs(del)
		}
	}
}

func (r *Runner) addJob(ctx context.Context, job Job) {
	if err := r.limiter.Acquire(ctx); err != nil {
		// job start canceled
		return
	}

	go func() {
		defer r.limiter.Release()

		if err := job.Start(ctx); err == nil {
			r.guard.Lock()
			defer r.guard.Unlock()

			r.running[job.Name] = job
			r.stopper[job.Name] = func() {
				job.Stop()
				delete(r.running, job.Name)
				delete(r.stopper, job.Name)
			}

			// additional check for ctx cancel in case of job not handle ctx correctly
			select {
			case <-ctx.Done():
				r.stopper[job.Name]()
			default:
			}

			return
		} else {
			// schedule start job retry
			go func() {
				select {
				case <-ctx.Done():
					// job start canceled
					return
				case <-time.After(r.cfg.RetryDelay):
					r.addJob(ctx, job)
				}
			}()
		}
	}()
}

func (r *Runner) deleteJobs(jobs []Job) {
	r.guard.Lock()
	defer r.guard.Unlock()

	for _, j := range jobs {
		if stopper, exist := r.stopper[j.Name]; exist {
			stopper()
		}
	}
}
