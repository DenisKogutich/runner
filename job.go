package runner

import (
	"context"
)

// Job is an abstraction describing some job to be started.
type Job interface {
	Start(context.Context) error
	Stop()
	ID() int64
}
