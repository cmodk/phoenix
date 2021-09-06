package phoenix

import (
	"fmt"
)

func StringSave(event interface{}) error {
	e := event.(StreamUpdated)

	value, ok := e.Value.(string)
	if !ok {
		log.WithField("stream", e).Debug("Cannot save string string as sample")
		return nil
	}

	if e.DeviceGuid == nil || *e.DeviceGuid == "" {
		if e.DeviceId == 0 {
			log.WithField("stream", e).Error("No device id or guid for string update, ignoring...")
			return nil
		}

		log.WithField("stream", e).Warningf("Need to look up device id %d, this will be a lot slower\n", e.DeviceId)
		d, err := phoenix.Devices.Get(DeviceCriteria{Id: e.DeviceId})
		if err != nil {
			return err
		}

		e.DeviceGuid = &(d.Guid)

	}

	if e.Timestamp == nil || e.Timestamp.IsZero() {
		return fmt.Errorf("Missing timestamp in string update")
	}

	if e.Code == "" {
		return fmt.Errorf("Missing code in string update")
	}

	query := phoenix.Cassandra.Query("INSERT INTO stream_strings (device,stream,timestamp,value) VALUES(?,?,?,?)",
		e.DeviceGuid,
		e.Code,
		e.Timestamp,
		value)
	if err := query.Exec(); err != nil {
		return err
	}

	return phoenix.Event.Publish(StringSaved(e))
}
