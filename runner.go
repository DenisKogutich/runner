package runner

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Runner struct {
	cfg     *Config
	wg      *sync.WaitGroup
	guard   *sync.RWMutex
	limiter *ConcurrentLimiter
	running map[string]Job
	stopper map[string]func()
	stop    chan struct{}
	tasks   *TasksChannel
	// expected count of running jobs (running + scheduled), used for logging
	expectedCount int32
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
	r.stop = make(chan struct{})
	r.tasks = NewTasksChannel()

	r.wg.Add(2)
	go r.eventLoop()
	go r.taskLoop()
}

func (r *Runner) Stop() {
	close(r.stop)
	r.wg.Wait()

	r.tasks.Close()
	r.stop = nil

	r.guard.Lock()
	defer r.guard.Unlock()

	for _, stopper := range r.stopper {
		stopper()
	}
}

func (r *Runner) Job(name string) (Job, bool) {
	r.guard.RLock()
	defer r.guard.RUnlock()

	j, exist := r.running[name]
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
				r.tasks.Put(NewAddTask(job))
			}
		case del := <-r.delete:
			atomic.AddInt32(&r.expectedCount, -int32(len(del)))
			for _, job := range del {
				r.tasks.Put(NewDeleteTask(job))
			}
		}
	}
}

func (r *Runner) taskLoop() {
	defer r.wg.Done()
	tasksChan := r.tasks.Chan()

	for {
		select {
		case <-r.stop:
			return
		case task := <-tasksChan:
			job := task.Job()

			// delete
			if task.Type() == Delete {
				r.deleteJob(job)
				continue
			}

			// add
			r.guard.RLock()
			_, skip := r.stopper[job.Name]
			r.guard.RUnlock()

			// job already running or scheduled for running
			if skip {
				continue
			}

			ctx, cancel := context.WithCancel(context.Background())
			r.guard.Lock()
			r.stopper[job.Name] = func() {
				cancel()
				delete(r.stopper, job.Name)
			}
			r.guard.Unlock()

			r.addJob(ctx, job)
		}
	}
}

func (r *Runner) addJob(ctx context.Context, job Job) {
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
			r.stopper[job.Name]()
			r.running[job.Name] = job
			r.stopper[job.Name] = func() {
				job.Stop()
				log.Printf("[INF] %q stopped\n", job)
				delete(r.running, job.Name)
				delete(r.stopper, job.Name)
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
					r.addJob(ctx, job)
				}
			}()
		}
	}()
}

func (r *Runner) deleteJob(job Job) {
	r.guard.Lock()
	defer r.guard.Unlock()

	if stopper, exist := r.stopper[job.Name]; exist {
		stopper()
	}
}
