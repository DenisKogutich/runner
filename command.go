package runner

type CommandType int

const (
	Stop CommandType = iota
	Start
)

// Command describes the action that needs to be performed on the Job.
type Command struct {
	j Job
	t CommandType
}

func NewStopCommand(j Job) *Command {
	return &Command{
		j: j,
		t: Stop,
	}
}

func NewStartCommand(j Job) *Command {
	return &Command{
		j: j,
		t: Start,
	}
}

func (c *Command) Type() CommandType {
	return c.t
}

func (c *Command) Job() Job {
	return c.j
}
