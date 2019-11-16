package main

import (
	"runner"
	"time"
)

func main() {
	cfg := &runner.Config{
		MaxParallelStarts: 10,
		RetryDelay:        time.Minute,
	}
	add, del := make(chan []runner.Job), make(chan []runner.Job)
	r := runner.NewRunner(cfg, add, del)

	r.Start()
	add <- []runner.Job{{Name: "1"}, {Name: "2"}, {Name: "3"}, {Name: "4"}, {Name: "5"}}
	time.Sleep(time.Second)
	r.Stop()
}
