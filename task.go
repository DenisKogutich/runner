package runner

type TaskType int

const (
	Delete TaskType = iota
	Add
)

type Task struct {
	j  Job
	tt TaskType
}

func NewDeleteTask(j Job) *Task {
	return &Task{
		j:  j,
		tt: Delete,
	}
}

func NewAddTask(j Job) *Task {
	return &Task{
		j:  j,
		tt: Add,
	}
}

func (t *Task) Type() TaskType {
	return t.tt
}

func (t *Task) Job() Job {
	return t.j
}
