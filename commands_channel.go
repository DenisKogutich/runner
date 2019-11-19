package runner

// CommandsChannel is an ordered channel of type Command. It behaves like unlimited buffered channel.
type CommandsChannel struct {
	stored []*Command
	in     chan *Command
	out    chan *Command
}

func NewCommandsChannel() *CommandsChannel {
	tc := &CommandsChannel{
		in:  make(chan *Command),
		out: make(chan *Command),
	}

	go tc.loop()

	return tc
}

func (cc *CommandsChannel) Chan() <-chan *Command {
	return cc.out
}

func (cc *CommandsChannel) Put(t *Command) {
	cc.in <- t
}

func (cc *CommandsChannel) Close() {
	close(cc.in)
}

func (cc *CommandsChannel) loop() {
	in := cc.in
	var out chan *Command
	var nextCmd *Command

	for in != nil || out != nil {
		select {
		case cmd, open := <-in:
			if !open {
				in = nil
				break
			}

			cc.stored = append(cc.stored, cmd)
		case out <- nextCmd:
			cc.stored = cc.stored[1:]
			nextCmd = nil
		}

		if len(cc.stored) > 0 && nextCmd == nil {
			nextCmd = cc.stored[0]
			out = cc.out
		} else if nextCmd == nil {
			out = nil
		}
	}

	close(cc.out)
}
