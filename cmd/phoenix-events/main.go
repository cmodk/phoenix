package main

import (
	"encoding/json"
	"flag"
	"reflect"
	"time"

	"github.com/cmodk/go-simpleflake"
	"github.com/cmodk/phoenix"
	"github.com/sirupsen/logrus"
)

var (
	app = phoenix.New()
	log = app.Logger

	debug = flag.Bool("debug", false, "Enable debug information")
)

func main() {
	flag.Parse()

	if *debug {
		app.Logger.Level = logrus.DebugLevel
	}

	app.HandleEvent(phoenix.DeviceNotificationCreated{}, updateLastKnownValue)
	app.HandleEvent(phoenix.DeviceNotificationCreated{}, splitBatchNotifications)
	app.HandleEvent(phoenix.StreamUpdated{}, saveSample)
	app.HandleEvent(phoenix.StreamUpdated{}, phoenix.StringSave)
	app.HandleEvent(phoenix.StreamUpdated{}, pipeEvents)
	app.HandleEvent(phoenix.DeviceNotificationCreated{}, pipeEvents)

	go app.Command.Listen()
	app.ListenEvents()
}

func pipeEvents(event interface{}) error {

	topics := []string{
		"fawkes.events",
	}

	app.Logger.WithField("event", event).WithField("type", reflect.TypeOf(event).String()).Debugf("Piping event")
	for _, topic := range topics {
		app.Event.PublishToTopic(topic, event)
	}

	return nil
}

func splitBatchNotifications(event interface{}) error {
	e := event.(phoenix.DeviceNotificationCreated)

	if e.Notification != "streams" {
		return nil
	}

	d, err := app.Devices.Get(phoenix.DeviceCriteria{Id: e.DeviceId})
	if err != nil {
		return err
	}

	var streams []phoenix.Stream

	if err := json.Unmarshal(e.Parameters, &streams); err != nil {
		return err
	}

	for _, s := range streams {
		if s.Timestamp == nil || s.Timestamp.IsZero() {
			now := time.Now()
			s.Timestamp = &now
		}
		log.WithField("stream", s).Debugf("Code: %s, Value: %f, Timestamp: %s",
			s.Code,
			s.Value,
			s.Timestamp)

		id := simpleflake.Next()

		stream_data, err := json.Marshal(s)
		if err != nil {
			return err
		}

		log.Debugf("Stream data: %s", string(stream_data))

		cmd := phoenix.DeviceNotificationCreate{
			Id:           id,
			DeviceGuid:   d.Guid,
			Notification: "stream",
			Timestamp:    *s.Timestamp,
			Parameters:   stream_data,
		}

		if err := app.Command.Create(cmd); err != nil {
			return err
		}

	}

	return nil
}
