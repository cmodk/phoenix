package phoenix

import (
	"encoding/json"
	"strconv"
	"time"
)

type Stream struct {
	Id         uint64  `db:"id" json:"id,omitempty" table:"device_streams"`
	DeviceId   uint64  `db:"device_id" json:"device_id,omitempty"`
	DeviceGuid *string `json:"device_guid,omitempty"`
	Code       string  `db:"code" json:"code"`
	//Timestamp  StreamTime  `db:"timestamp" json:"timestamp,omitempty"`
	Timestamp *time.Time  `db:"timestamp" json:"timestamp,omitempty"`
	Value     interface{} `db:"value" json:"value"`
}

type LowMemoryDeviceStream struct {
	Stream
	Timestamp StreamTime `db:"timestamp" json:"timestamp,omitempty"`
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

type StreamTime time.Time

func (t *StreamTime) UnmarshalJSON(b []byte) (err error) {
	var epoch int64

	if epoch, err = strconv.ParseInt(string(b), 10, 64); err == nil {
		//Timestamp was a unix time!
		*t = StreamTime(time.Unix(epoch, 0))
		return nil
	}

	return nil

}

func (lmStream *LowMemoryDeviceStream) ToStream() Stream {

	//Need a pointer for the timestamp
	t := time.Time(lmStream.Timestamp)

	var stream Stream

	stream.Id = lmStream.Id
	stream.Code = lmStream.Code
	stream.DeviceId = lmStream.DeviceId
	stream.DeviceGuid = lmStream.DeviceGuid
	stream.Timestamp = &t
	stream.Value = lmStream.Value

	return stream
}
