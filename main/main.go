package main

import (
	"context"
	"errors"
	"fmt"
	"runner"
	"time"
)

// exampleJob implements exampleJob.
type exampleJob struct {
	name string
	id   int64
}

func (j exampleJob) Start(context.Context) error {
	if j.name == "fail" {
		return errors.New("err")
	}

	return nil
}

func (j exampleJob) ID() int64 { return j.id }

func (j exampleJob) Stop() {}

func (j exampleJob) String() string { return fmt.Sprintf("job %q", j.name) }

func main() {
	cfg := &runner.Config{
		MaxParallelStarts: 10,
		RetryDelay:        time.Second,
	}
	add, del := make(chan []runner.Job), make(chan []runner.Job)
	r := runner.NewRunner(cfg, add, del)

	go func() {
		for {
			j, ok := r.Job(1)
			fmt.Println(j, ok)
			time.Sleep(250 * time.Millisecond)
		}
	}()

	r.Start()
	add <- []runner.Job{
		exampleJob{name: "1", id: 1},
		exampleJob{name: "2", id: 2},
		exampleJob{name: "fail", id: 3},
		exampleJob{name: "4", id: 4},
		exampleJob{name: "5", id: 5},
	}

	time.AfterFunc(3*time.Second, func() {
		del <- []runner.Job{exampleJob{name: "2", id: 2}, exampleJob{name: "fail", id: 3}}

		time.AfterFunc(5*time.Second, func() {
			add <- []runner.Job{exampleJob{name: "6", id: 6}}

			time.AfterFunc(2*time.Second, func() {
				del <- []runner.Job{exampleJob{name: "1", id: 1}}
			})
		})
	})

	time.Sleep(time.Minute)
}
