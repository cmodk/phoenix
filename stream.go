package phoenix

import (
	"encoding/json"
	"time"
)

type Stream struct {
	Id         uint64      `db:"id" json:"id,omitempty" table:"device_streams"`
	DeviceId   uint64      `db:"device_id" json:"device_id,omitempty"`
	DeviceGuid *string     `json:"device_guid,omitempty"`
	Code       string      `db:"code" json:"code"`
	Timestamp  *time.Time  `db:"timestamp" json:"timestamp,omitempty"`
	Value      interface{} `db:"value" json:"value"`
}

func (s *Stream) Notification() DeviceNotification {

	raw_stream, err := json.Marshal(*s)
	if err != nil {
		panic(err)
	}

	n := DeviceNotification{
		Notification: "stream",
		Timestamp:    time.Now(),
		Parameters:   json.RawMessage(raw_stream),
	}

	return n
}

type StreamCriteria struct {
	DeviceId uint64 `schema:"device_id" db:"device_id"`
	Code     string `schema:"code" db:"code"`

	Limit int `schema:"limit"`
}
