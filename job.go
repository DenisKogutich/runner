package runner

import (
	"context"
	"fmt"
)

type Job struct {
	Name string
}

func (j Job) Start(context.Context) error {
	fmt.Printf("%v started\n", j)
	return nil
}

func (j Job) Stop() {
	fmt.Printf("%v stopped\n", j)
}

func (j Job) String() string {
	return fmt.Sprintf("job %q", j.Name)
}
