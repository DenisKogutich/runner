package runner

// TasksChannel is an ordered channel with tasks.
type TasksChannel struct {
	tasks []*Task
	in    chan *Task
	out   chan *Task
}

func NewTasksChannel() *TasksChannel {
	tc := &TasksChannel{
		in:  make(chan *Task),
		out: make(chan *Task),
	}

	go tc.loop()

	return tc
}

func (tc *TasksChannel) Put(t *Task) {
	tc.in <- t
}

func (tc *TasksChannel) Chan() <-chan *Task {
	return tc.out
}

func (tc *TasksChannel) Close() {
	close(tc.in)
}

func (tc *TasksChannel) loop() {
	in := tc.in
	var out chan *Task
	var nextTask *Task

	for in != nil || out != nil {
		select {
		case task, ok := <-in:
			if !ok {
				in = nil
				continue
			}

			tc.tasks = append(tc.tasks, task)
		case out <- nextTask:
			tc.tasks = tc.tasks[1:]
		}

		if len(tc.tasks) > 0 {
			nextTask = tc.tasks[0]
			out = tc.out
		} else {
			out = nil
		}
	}

	close(tc.out)
}
