package main

import (
	"runner"
	"time"
)

func main() {
	cfg := &runner.Config{
		MaxParallelStarts: 10,
		RetryDelay:        time.Second,
	}
	add, del := make(chan []runner.Job), make(chan []runner.Job)
	r := runner.NewRunner(cfg, add, del)

	r.Start()
	add <- []runner.Job{{Name: "1"}, {Name: "2"}, {Name: "fail"}, {Name: "4"}, {Name: "5"}}

	time.AfterFunc(3*time.Second, func() {
		del <- []runner.Job{{Name: "2"}, {Name: "fail"}}

		time.AfterFunc(5*time.Second, func() {
			add <- []runner.Job{{Name: "6"}}
		})
	})

	time.Sleep(time.Minute)
}
