package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmodk/phoenix"
)

func updateLastKnownValue(event interface{}) error {
	e := event.(phoenix.DeviceNotificationCreated)

	if e.Notification != "stream" {
		//Not a stream, ignore
		return nil
	}

	d, err := app.Devices.Get(phoenix.DeviceCriteria{
		Id: e.DeviceId,
	})
	if err != nil {
		return err
	}

	var stream phoenix.Stream
	if err := json.Unmarshal(e.Parameters, &stream); err != nil {
		log.Printf("Ignoring bad timestamp for now: %v", err)
		return nil
	}

	if stream.Timestamp == nil || stream.Timestamp.IsZero() {
		log.Printf("Missing timestamp for string, using notification time: %s\n", e.Timestamp.Format(time.RFC3339))
		stream.Timestamp = &e.Timestamp
	}

	if stream.Timestamp.Unix() < 0 {
		log.Errorf("Timestamp is not valid: %s\n", stream.Timestamp.Format(time.RFC3339))

		//Ignore this for now, nothing to do and nsq will just requeue...
		return nil
	}

	if err := d.StreamUpdate(stream); err != nil {
		return err
	}

	app.Logger.WithField("stream", e).Debug("Updating value")
	stream.DeviceId = d.Id
	stream.DeviceGuid = &(d.Guid)

	return app.Event.Publish(phoenix.StreamUpdated(stream))
}

func saveSample(event interface{}) error {
	e := event.(phoenix.StreamUpdated)

	value, ok := e.Value.(float64)
	if !ok {
		app.Logger.WithField("stream", e).Debug("Cannot save string stream as sample")
		return nil
	}

	if e.DeviceGuid == nil || *e.DeviceGuid == "" {
		if e.DeviceId == 0 {
			app.Logger.WithField("stream", e).Error("No device id or guid for stream update, ignoring...")
			return nil
		}

		app.Logger.WithField("stream", e).Warningf("Need to look up device id %d, this will be a lot slower\n", e.DeviceId)
		d, err := app.Devices.Get(phoenix.DeviceCriteria{Id: e.DeviceId})
		if err != nil {
			return err
		}

		e.DeviceGuid = &(d.Guid)

	}

	if e.Timestamp == nil || e.Timestamp.IsZero() {
		return fmt.Errorf("Missing timestamp in stream update")
	}

	if e.Code == "" {
		return fmt.Errorf("Missing code in stream update")
	}

	query := app.Cassandra.Query("INSERT INTO samples (device,stream,timestamp,value) VALUES(?,?,?,?)",
		e.DeviceGuid,
		e.Code,
		e.Timestamp,
		value)
	if err := query.Exec(); err != nil {
		return err
	}

	s := phoenix.Sample{
		Device:    *e.DeviceGuid,
		Stream:    e.Code,
		Timestamp: *e.Timestamp,
		Value:     &value,
	}

	if e.DeviceId != 0 {
		s.DeviceId = &(e.DeviceId)
	}

	return app.Event.Publish(phoenix.SampleSaved(s))
}
