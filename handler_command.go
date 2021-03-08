package phoenix

import (
	"log"
)

func deviceNotificationCreate(command interface{}) error {
	cmd := command.(DeviceNotificationCreate)

	log.Printf("Creating device notification: %v\n", cmd)

	d, err := phoenix.Devices.Get(DeviceCriteria{
		Guid: cmd.DeviceGuid,
	})
	if err != nil {
		return err
	}

	log.Printf("Found device: %v\n", d)

	n := DeviceNotification{
		Id:           cmd.Id,
		Notification: cmd.Notification,
		Timestamp:    cmd.Timestamp,
		Parameters:   cmd.Parameters,
	}

	if err := d.NotificationInsert(&n); err != nil {
		return err
	}

	n.DeviceId = d.Id

	return phoenix.Event.Publish(DeviceNotificationCreated(n))

}
