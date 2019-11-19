package runner

import "context"

// ConcurrentLimiter limits parallel actions.
type ConcurrentLimiter struct {
	quota chan struct{}
}

func NewConcurrentLimiter(limit uint) *ConcurrentLimiter {
	return &ConcurrentLimiter{
		quota: make(chan struct{}, limit),
	}
}

func (cl *ConcurrentLimiter) Acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case cl.quota <- struct{}{}:
		return nil
	}
}

func (cl *ConcurrentLimiter) Release() {
	select {
	case <-cl.quota:
	default:
	}
}
