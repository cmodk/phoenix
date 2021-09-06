package phoenix

func deviceNotificationCreate(command interface{}) error {
	cmd := command.(DeviceNotificationCreate)

	d, err := phoenix.Devices.Get(DeviceCriteria{
		Guid: cmd.DeviceGuid,
	})
	if err != nil {
		return err
	}

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
