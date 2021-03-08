package phoenix

import (
	"github.com/cmodk/phoenix/app"
)

var (
	phoenix *Phoenix
)

type DeviceEvent struct {
	DeviceId uint64
}

type DeviceOnline DeviceEvent

type Phoenix struct {
	*app.App
	Devices *Devices
}

func New() *Phoenix {
	phoenix = &Phoenix{
		App: app.New(),
	}

	phoenix.ConnectMariadb()

	phoenix.Devices = NewDevices(phoenix)

	phoenix.HandleCommand(DeviceNotificationCreate{}, deviceNotificationCreate)

	return phoenix
}
