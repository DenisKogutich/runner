package runner

import (
	"context"
	"errors"
	"fmt"
)

type Job struct {
	Name string
}

func (j Job) Start(context.Context) error {
	if j.Name == "fail" {
		return errors.New("err")
	}

	return nil
}

func (j Job) Stop() {}

func (j Job) String() string {
	return fmt.Sprintf("job %q", j.Name)
}
