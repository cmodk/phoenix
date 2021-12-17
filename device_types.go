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

type DeviceCommand struct {
	Id         uint64           `db:"id" json:"id" table:"device_commands"`
	DeviceId   uint64           `db:"device_id" json:"-"`
	DeviceGuid string           `db:"device_guid" json:"device_guid"`
	Command    string           `db:"command" json:"command"`
	Pending    bool             `db:"pending" json:"pending"`
	Created    time.Time        `db:"created" json:"created"`
	Parameters *json.RawMessage `db:"parameters" json:"parameters"`
	Response   *json.RawMessage `db:"response" json:"response"`
}
