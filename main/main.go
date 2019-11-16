package main

import (
	"runner"
	"time"
)

func main() {
	cfg := &runner.RunnerConfig{
		MaxParallelStarts: 10,
		RetryDelay:        time.Minute,
	}
	add, del := make(chan []runner.Job), make(chan []runner.Job)
	runner := runner.NewRunner(cfg, add, del)

	runner.Start()
	add <- []runner.Job{{Name: "1"}, {Name: "2"}, {Name: "3"}, {Name: "4"}, {Name: "5"}}
	runner.Stop()

	time.Sleep(time.Minute)
}
