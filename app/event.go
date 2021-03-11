package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/nsqio/go-nsq"
)

const (
	EventBusChannelSize = 1000
)

type EventHandlerFunc func(event interface{}) error

type EventHandler struct {
	f EventHandlerFunc
	t reflect.Type
}

type EventBusConfig struct {
	NumHandlers int `yaml:"NumHandlers"`
	ListenName  *string
}

type EventBus struct {
	app      *App
	queue    chan interface{}
	handlers map[string][]EventHandler

	config *EventBusConfig
}

func NewEventBus(app *App) *EventBus {

	return &EventBus{
		app:      app,
		queue:    make(chan interface{}, EventBusChannelSize),
		handlers: make(map[string][]EventHandler),
		config:   app.Config.EventBus,
	}
}

type NsqEvent struct {
	Event   string          `json:"e"`
	Message json.RawMessage `json:"msg"`
}

func (bus *EventBus) SetListenName(name string) {
	bus.config.ListenName = &name
}

func (bus *EventBus) HandleMessage(m *nsq.Message) error {
	var e NsqEvent

	if err := json.Unmarshal(m.Body, &e); err != nil {
		return err
	}

	handlers, ok := bus.handlers[e.Event]
	if ok {
		//Type is the same for all
		msg := reflect.New(handlers[0].t).Interface()

		if err := json.Unmarshal(e.Message, msg); err != nil {
			return err
		}

		event := reflect.ValueOf(msg).Elem().Interface()

		for _, h := range handlers {
			if err := h.f(event); err != nil {
				event_data, _ := json.Marshal(event)
				bus.app.Logger.WithField("event", string(event_data)).Error("Error handling event")
				return err
			}
		}
	}

	return nil
}

func (bus *EventBus) Listen() {
	if bus.config == nil {
		panic(fmt.Errorf("Missing config for eventbus\n"))
	}
	for k, handlers := range bus.handlers {
		log.Printf("%s has %d handlers registered\n", k, len(handlers))
		for _, h := range handlers {
			log.Printf("  Type: %s\n", h.t.String())
		}
	}

	application := filepath.Base(os.Args[0])

	if bus.config.ListenName != nil {
		application = *bus.config.ListenName
	}

	config := nsq.NewConfig()
	consumer, err := nsq.NewConsumer(*bus.app.Config.NsqTopic, application, config)
	if err != nil {
		log.Fatal(err)
	}

	// Set the Handler for messages received by this Consumer. Can be called multiple times.
	// See also AddConcurrentHandlers.
	consumer.AddConcurrentHandlers(bus, bus.config.NumHandlers)

	// Use nsqlookupd to discover nsqd instances.
	// See also ConnectToNSQD, ConnectToNSQDs, ConnectToNSQLookupds.
	err = consumer.ConnectToNSQLookupd(*bus.app.Config.NsqLookupd)
	if err != nil {
		log.Fatal(err)
	}

	// wait for signal to exit
	sigChan := make(chan os.Signal, 1)
	<-sigChan

	// Gracefully stop the consumer.
	consumer.Stop()
}

func (bus *EventBus) Handle(event interface{}, handler EventHandlerFunc) {
	event_id := getEventId(event)
	log.Printf("Registering event: %s\n", event_id)
	h := EventHandler{handler, reflect.TypeOf(event)}
	bus.handlers[event_id] = append(bus.handlers[event_id], h)
}

func (bus *EventBus) Publish(event interface{}) error {

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg, err := json.Marshal(NsqEvent{
		Event:   getEventId(event),
		Message: json.RawMessage(data),
	})
	if err != nil {
		return err
	}

	return bus.app.NsqProducer.Publish(*bus.app.Config.NsqTopic, msg)
}

func (bus *EventBus) PublishToTopic(topic string, event interface{}) error {

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg, err := json.Marshal(NsqEvent{
		Event:   getEventId(event),
		Message: json.RawMessage(data),
	})
	if err != nil {
		return err
	}

	return bus.app.NsqProducer.Publish(topic, msg)
}
