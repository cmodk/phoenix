package app

const (
	CommandBusChannelSize = 1000
)

type CommandHandler func(cmd interface{}) error

type CommandBus struct {
	app      *App
	queue    chan interface{}
	handlers map[string][]CommandHandler
}

func NewCommandBus(app *App) *CommandBus {
	return &CommandBus{
		app:      app,
		queue:    make(chan interface{}, CommandBusChannelSize),
		handlers: make(map[string][]CommandHandler),
	}
}

func (bus *CommandBus) Handle(command interface{}, handler CommandHandler) {
	cmd_id := getEventId(command)
	bus.handlers[cmd_id] = append(bus.handlers[cmd_id], handler)
}

func (bus *CommandBus) Listen() {

	log.Debugf("Listening for commands\n")

	for {
		cmd := <-bus.queue
		cmd_id := getEventId(cmd)

		handlers, ok := bus.handlers[cmd_id]
		if ok {
			for _, handler := range handlers {
				if err := handler(cmd); err != nil {
					bus.app.Logger.WithField("error", err).Errorf("Error handling command: %s -> %v\n", cmd_id, cmd)
				}
			}
		}

	}
}

func (bus *CommandBus) Create(cmd interface{}) error {
	bus.queue <- cmd
	return nil
}
