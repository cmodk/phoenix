package phoenix

import (
	"time"
)

type Sample struct {
	DeviceId  *uint64   `json:"-"`
	Device    string    `json:"device"`
	Stream    string    `json:"stream"`
	Timestamp time.Time `json:"timestamp"`

	//For raw samples
	Value *float64 `json:"value,omitempty"`

	//For aggregated
	Average *float64 `json:"average,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Count   *int     `json:"count,omitempty"`
}

type SampleCriteria struct {
	Streams   []string  `schema:"stream"`
	From      time.Time `schema:"from"`
	To        time.Time `schema:"to"`
	Frequency string    `schema:"frequency"`

	Limit int `schema:"limit"`
}
