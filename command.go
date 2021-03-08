package phoenix

import (
	"encoding/json"
	"time"
)

type DeviceNotificationCreate struct {
	Id           uint64
	DeviceGuid   string
	Notification string
	Timestamp    time.Time
	Parameters   json.RawMessage
}
