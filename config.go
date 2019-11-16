package runner

import "time"

type Config struct {
	MaxParallelStarts uint
	RetryDelay        time.Duration
}
