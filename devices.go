package phoenix

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cmodk/go-simpleflake"
	"github.com/cmodk/phoenix/app"
	"github.com/gocql/gocql"
)

type Devices struct {
	db *app.Database
	ca *gocql.Session
}

func NewDevices(app *Phoenix) *Devices {

	return &Devices{app.Database, app.Cassandra}
}

func (devices *Devices) List(c DeviceCriteria) (*[]Device, error) {
	var ds []Device
	err := devices.db.Match(&ds, "devices", c)

	if err != nil {
		return nil, err
	}

	return &ds, nil

}

func (devices *Devices) Get(c DeviceCriteria) (*Device, error) {
	var d Device
	err := devices.db.MatchOne(&d, "devices", c)

	if err != nil {
		return nil, err
	}

	d.db = devices.db
	d.ca = devices.ca

	return &d, nil

}

type Device struct {
	db      *app.Database
	ca      *gocql.Session
	Id      uint64    `db:"id" json:"id"`
	Guid    string    `db:"guid" json:"guid"`
	Created time.Time `db:"created" json:"created"`
	Token   *string   `db:"token" json:"-"`
	Online  bool      `db:"online" json:"online"`
}

func (d *Device) UpdateOnlineStatus(status bool) error {
	return d.Update("online", status)
}

func (d *Device) Update(column string, value interface{}) error {
	_, err := d.db.Exec(fmt.Sprintf("UPDATE devices SET %s = ? WHERE id = ?", column), value, d.Id)
	return err
}

func (d *Device) NotificationInsert(n *DeviceNotification) error {
	if n.Id == 0 {
		n.Id = simpleflake.Next()
	}
	n.DeviceId = d.Id

	query := d.ca.Query("INSERT INTO notifications (id,device,timestamp,notification,parameters) VALUES(?,?,?,?,?)",
		n.Id,
		d.Guid,
		n.Timestamp,
		n.Notification,
		n.Parameters)

	return query.Exec()
}

func (d *Device) NotificationList(c DeviceNotificationCriteria) ([]DeviceNotification, error) {

	var notifications []DeviceNotification

	query := d.ca.Query("SELECT id,timestamp,notification,parameters FROM notifications WHERE device = ?", d.Guid)

	log.Printf("Executing cassandra query: %s\n", query.String())
	iter := query.Iter()
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		notification := DeviceNotification{
			Id:           uint64(row["id"].(int64)),
			DeviceId:     d.Id,
			Notification: row["notification"].(string),
			Timestamp:    row["timestamp"].(time.Time),
			Parameters:   json.RawMessage(row["parameters"].(string)),
		}
		notifications = append(notifications, notification)
	}
	iter.Close()

	return notifications, nil
}

func (d *Device) SampleList(c SampleCriteria) ([]Sample, error) {
	max_samples := 1000000
	var samples []Sample

	if len(c.Streams) == 0 {
		streams, err := d.StreamList(StreamCriteria{})
		if err != nil {
			return []Sample{}, err
		}

		for _, s := range *streams {
			c.Streams = append(c.Streams, s.Code)
		}
	}

	if c.Limit == 0 {
		c.Limit = max_samples
	}

	if c.Limit*len(c.Streams) > max_samples {
		new_limit := max_samples / len(c.Streams)
		phoenix.Logger.Warningf("Request for %d sample streams with limit %d, reducing query size to %d samples pr stream",
			len(c.Streams),
			c.Limit,
			new_limit)
		c.Limit = new_limit
		if c.Limit == 0 {
			return []Sample{}, fmt.Errorf("Requested %d sample streams, which is higher than total limit of samples %d", len(c.Streams), max_samples)
		}

	}

	if c.Frequency == "raw" {
		for _, s := range c.Streams {
			query := d.ca.Query("SELECT * FROM samples WHERE device = ? AND stream = ? AND timestamp > ? and timestamp < ?  LIMIT ?",
				d.Guid,
				s,
				c.From,
				c.To,
				c.Limit)
			log.Printf("Executing cassandra query: %s\n", query.String())
			iter := query.Iter()
			for {
				row := make(map[string]interface{})
				if !iter.MapScan(row) {
					break
				}
				value := row["value"].(float64)
				sample := Sample{
					Device:    row["device"].(string),
					Stream:    row["stream"].(string),
					Timestamp: row["timestamp"].(time.Time),
					Value:     &value,
				}
				samples = append(samples, sample)
				if len(samples) == max_samples {
					break
				}
			}
			iter.Close()

			if len(samples) == max_samples {
				break
			}

		}
	} else {
		//		table := fmt.Sprintf("samples_%s", c.Frequency)
		for _, s := range c.Streams {
			q := fmt.Sprintf("SELECT * FROM samples_%s WHERE device = ? AND stream = ? AND timestamp > ? and timestamp < ?  LIMIT ?", c.Frequency)
			query := d.ca.Query(q,
				d.Guid,
				s,
				c.From,
				c.To,
				c.Limit)
			log.Printf("Executing cassandra query: %s\n", query.String())
			iter := query.Iter()
			for {
				row := make(map[string]interface{})
				if !iter.MapScan(row) {
					break
				}
				average := row["average"].(float64)
				max := row["max"].(float64)
				min := row["min"].(float64)
				count := row["count"].(int)
				sample := Sample{
					Device:    row["device"].(string),
					Stream:    row["stream"].(string),
					Timestamp: row["timestamp"].(time.Time),
					Average:   &average,
					Max:       &max,
					Min:       &min,
					Count:     &count,
				}
				samples = append(samples, sample)
				if len(samples) == max_samples {
					break
				}
			}
			iter.Close()

			if len(samples) == max_samples {
				break
			}

		}

	}
	return samples, nil
}

func (d *Device) StreamUpdate(s Stream) error {

	current, err := d.StreamGet(StreamCriteria{
		DeviceId: d.Id,
		Code:     s.Code,
	})
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if current == nil {
		//Not found
		s.DeviceId = d.Id
		return d.db.Insert(s, "device_streams")
	}

	if current.Timestamp == nil || (s.Timestamp != nil && s.Timestamp.After(*current.Timestamp)) {
		_, err := d.db.Exec("UPDATE device_streams SET value=?, timestamp=? WHERE device_id=? and code=?",
			s.Value,
			s.Timestamp,
			d.Id,
			s.Code)
		return err
	}

	return nil
}

func (d *Device) StreamList(c StreamCriteria) (*([]Stream), error) {
	var streams []Stream
	if err := d.db.Match(&streams, "device_streams", c); err != nil {
		return nil, err
	}

	return &streams, nil
}
func (d *Device) StreamGet(c StreamCriteria) (*Stream, error) {
	var s Stream
	if err := d.db.MatchOne(&s, "device_streams", c); err != nil {
		return nil, err
	}

	return &s, nil
}

type DeviceCriteria struct {
	Id      uint64    `schema:"id"`
	Guid    string    `schema:"guid"`
	Token   string    `schema:"token"`
	Created time.Time `schema:"created"`

	Limit int `schema:"limit"`
}

type DeviceNotificationCriteria struct {
	Limit int `schema:"limit"`
}

type DeviceCommandCriteria struct {
	DeviceId uint64 `schema:"device_id"`
	Pending  bool   `schema:"pending"`

	Limit int `schema:"limit"`
}

func (d *Device) CommandInsert(command *DeviceCommand) error {

	command.Created = time.Now().UTC()
	command.Id = simpleflake.Next()
	command.DeviceGuid = d.Guid
	command.DeviceId = d.Id
	command.Pending = true

	return d.db.Insert(*command, "device_commands")
}

func (d *Device) CommandSent(cmd *DeviceCommand) error {
	_, err := d.db.Exec("UPDATE device_commands SET pending=0 WHERE device_id=? and id =?", d.Id, cmd.Id)

	return err
}

func (d *Device) CommandsPending() ([]DeviceCommand, error) {

	c := DeviceCommandCriteria{
		DeviceId: d.Id,
		Pending:  true,
	}

	var commands []DeviceCommand

	if err := d.db.Match(&commands, "device_commands", c); err != nil {
		return nil, err
	}

	return commands, nil

}
