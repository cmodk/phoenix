package app

import (
	"log"
)

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
	log.Printf("Registering command for id: %s\n", cmd_id)
	bus.handlers[cmd_id] = append(bus.handlers[cmd_id], handler)
}

func (bus *CommandBus) Listen() {

	log.Printf("Listening for commands\n")

	for {
		cmd := <-bus.queue
		cmd_id := getEventId(cmd)
		log.Printf("Got command: %s -> %v\n", cmd_id, cmd)

		handlers, ok := bus.handlers[cmd_id]
		if ok {
			log.Printf("Found handler for %s\n", cmd_id)
			for _, handler := range handlers {
				if err := handler(cmd); err != nil {
					log.Printf("Error handling command: %s -> %v\n", cmd_id, cmd)
					bus.app.Logger.WithField("error", err).Errorf("Error handling command: %s -> %v\n", cmd_id, cmd)
				}
			}
		}

	}
}

func (bus *CommandBus) Create(cmd interface{}) error {
	log.Printf("Inserting command: %v\n", cmd)
	bus.queue <- cmd

	return nil
}
