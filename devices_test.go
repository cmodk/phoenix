package phoenix

import (
	"testing"
	"time"
)

var (
	test_app = New()
)

func TestNotificationWrite(t *testing.T) {

	now := time.Now()
	s := Stream{
		Name:      "test.stream",
		Timestamp: &now,
		Value:     12.34,
	}

	d, err := test_app.Devices.Get(DeviceCriteria{
		Guid: "test_device",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.NotificationInsert(s.Notification())
	if err != nil {
		t.Fatal(err)
	}

}
