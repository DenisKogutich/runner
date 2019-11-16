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
	tasks   *TasksChannel

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
	r.tasks = NewTasksChannel()

	r.wg.Add(2)
	go r.eventLoop()
	go r.taskLoop()
}

func (r *Runner) Stop() {
	close(r.stopC)
	r.wg.Wait()

	r.tasks.Close()
	r.stopC = nil

	r.guard.Lock()
	defer r.guard.Unlock()

	for _, stopper := range r.stopper {
		stopper()
	}
}

func (r *Runner) eventLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopC:
			return
		case add := <-r.add:
			for _, job := range add {
				r.tasks.Put(NewAddTask(job))
			}
		case del := <-r.delete:
			for _, job := range del {
				r.tasks.Put(NewDeleteTask(job))
			}
		}
	}
}

func (r *Runner) taskLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.stopC:
			return
		case task := <-r.tasks.Chan():
			job := task.Job()

			// delete
			if task.Type() == Delete {
				r.deleteJob(job)
				return
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
		return
	}

	go func() {
		defer r.limiter.Release()

		if err := job.Start(ctx); err == nil {
			// additional check for ctx cancel in case of job not handle ctx correctly
			select {
			case <-ctx.Done():
				job.Stop()
				return
			default:
			}

			r.guard.Lock()
			defer r.guard.Unlock()

			r.running[job.Name] = job
			r.stopper[job.Name] = func() {
				job.Stop()
				delete(r.running, job.Name)
				delete(r.stopper, job.Name)
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

func (r *Runner) deleteJob(job Job) {
	r.guard.Lock()
	defer r.guard.Unlock()

	if stopper, exist := r.stopper[job.Name]; exist {
		stopper()
	}
}
