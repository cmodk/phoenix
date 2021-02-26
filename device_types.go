package phoenix

import (
	"encoding/json"
	"time"
)

type DeviceNotification struct {
	Id           uint64          `db:"id" json:"id" table:"device_notifications"`
	DeviceId     uint64          `db:"device_id" json:"device_id"`
	Notification string          `db:"notification" json:"notification"`
	Timestamp    time.Time       `db:"timestamp" json:"timestamp"`
	Parameters   json.RawMessage `db:"parameters" json:"parameters"`
}
