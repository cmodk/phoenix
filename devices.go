package phoenix

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
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

	for i, _ := range ds {
		d := &(ds[i])
		d.db = devices.db
		d.ca = devices.ca
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
	db              *app.Database
	ca              *gocql.Session
	Id              uint64     `db:"id" json:"id"`
	Guid            string     `db:"guid" json:"guid"`
	Created         time.Time  `db:"created" json:"created"`
	Token           *string    `db:"token" json:"-"`
	TokenExpiration *time.Time `db:"token_expiration" json:"token_expiration"`
	Online          bool       `db:"online" json:"online"`
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

	if err := query.Exec(); err != nil {
		//Must not happen!
		log.WithField("error", err).Error("Could not insert device notification")
		panic(err)
	}

	return nil
}

func (d *Device) NotificationUpdateParameters(n *DeviceNotification) error {

	query := d.ca.Query("UPDATE notifications SET parameters = ? WHERE device = ? AND timestamp = ? AND id = ?",
		n.Parameters,
		d.Guid,
		n.Timestamp,
		n.Id)

	if err := query.Exec(); err != nil {
		//Must not happen!
		log.WithField("error", err).Error("Could not update device notification")
		panic(err)
	}

	return nil
}
func (d *Device) NotificationList(c DeviceNotificationCriteria) ([]DeviceNotification, error) {

	var notifications []DeviceNotification

	q := squirrel.Select("id,timestamp,notification,parameters").
		From("notifications").
		Where(squirrel.Eq{"device": d.Guid}).
		Where(squirrel.Gt{"timestamp": c.From}).
		Where(squirrel.Lt{"timestamp": c.To})

	if len(c.Notification) > 0 {
		q = q.Where(squirrel.Eq{"notification": c.Notification})

	}

	cql, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	query := d.ca.Query(cql, args...)
	log.Debugf("Executed cassandra query: %s\n", query.String())

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
	err = iter.Close()

	return notifications, err
}

func (d *Device) NotificationGet(c DeviceNotificationCriteria) (*DeviceNotification, error) {

	var notifications []DeviceNotification

	query := d.ca.Query("SELECT id,timestamp,notification,parameters FROM notifications WHERE device = ? AND timestamp >= ? AND timestamp <= ? AND id = ?",
		d.Guid,
		d.Created,
		time.Now(),
		c.Id)

	log.Debugf("Executing cassandra query: %s\n", query.String())
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
	err := iter.Close()
	if err != nil {
		return nil, err
	}

	if len(notifications) == 0 {
		return nil, fmt.Errorf("No notifications found with id %d\n", c.Id)
	}

	if len(notifications) > 1 {
		return nil, fmt.Errorf("Multiple notifications found with id %d, count = %d", c.Id, len(notifications))
	}

	return &notifications[0], nil
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
			log.Debugf("Executing cassandra query: %s\n", query.String())
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
			log.Debugf("Executing cassandra query: %s\n", query.String())
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

func (d *Device) StreamValueList(c SampleCriteria) ([]StreamStringValue, error) {
	max_samples := 1000000
	var values []StreamStringValue

	if len(c.Streams) != 1 {
		return values, fmt.Errorf("Not possible to request string values from multiple streams")
	}

	if c.Limit == 0 {
		c.Limit = max_samples
	}

	for _, s := range c.Streams {
		query := d.ca.Query("SELECT timestamp,value FROM stream_strings WHERE device = ? AND stream = ? AND timestamp > ? and timestamp < ?  LIMIT ?",
			d.Guid,
			s,
			c.From,
			c.To,
			c.Limit)
		log.Debugf("Executing cassandra query: %s\n", query.String())
		iter := query.Iter()
		for {
			row := make(map[string]interface{})
			if !iter.MapScan(row) {
				break
			}
			value := StreamStringValue{
				Timestamp: row["timestamp"].(time.Time),
				Value:     row["value"].(string),
			}
			values = append(values, value)
			if len(values) == max_samples {
				break
			}
		}
		iter.Close()

		if len(values) == max_samples {
			break
		}

	}
	return values, nil
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
		return d.db.Insert(&s, "device_streams")
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
	c.DeviceId = d.Id
	var streams []Stream
	if err := d.db.Match(&streams, "device_streams", c); err != nil {
		return nil, err
	}

	return &streams, nil
}
func (d *Device) StreamGet(c StreamCriteria) (*Stream, error) {
	c.DeviceId = d.Id
	var s Stream
	if err := d.db.MatchOne(&s, "device_streams", c); err != nil {
		return nil, err
	}

	return &s, nil
}

type DeviceCriteria struct {
	Id      uint64    `schema:"id" db:"id"`
	Guid    string    `schema:"guid" db:"guid"`
	Token   string    `schema:"token" db:"token"`
	Created time.Time `schema:"created" db:"created"`

	Limit int `schema:"limit"`
}

type DeviceNotificationCriteria struct {
	From         time.Time `schema:"start"`
	To           time.Time `schema:"end"`
	Id           uint64    `schema:"id"`
	Notification string    `schema:"notification"`

	Limit int `schema:"limit"`
}

type DeviceCommandCriteria struct {
	Id       uint64 `schema:"id" db:"id"`
	DeviceId uint64 `schema:"device_id" db:"device_id"`
	Pending  bool   `schema:"pending" db:"pending"`

	Limit int `schema:"limit"`
}

func (d *Device) CommandInsert(command *DeviceCommand) error {

	command.Created = time.Now().UTC()
	command.Id = simpleflake.Next()
	command.DeviceGuid = d.Guid
	command.DeviceId = d.Id
	command.Pending = true

	return d.db.Insert(command, "device_commands")
}

func (d *Device) CommandGet(c DeviceCommandCriteria) (*DeviceCommand, error) {
	c.DeviceId = d.Id

	var command DeviceCommand
	if err := d.db.MatchOne(&command, "device_commands", c); err != nil {
		return nil, err
	}

	return &command, nil

}

func (d *Device) CommandResponse(cmd *DeviceCommand, value interface{}) error {
	response, err := json.Marshal(value)
	if err != nil {
		return err
	}

	query, args, err := squirrel.Update("device_commands").Set("response", response).Where(squirrel.Eq{"id": cmd.Id}).ToSql()
	if err != nil {
		return err
	}

	phoenix.Logger.Debugf("Executing: %s -> %v", query, args)
	_, err = d.db.Exec(query, args...)
	return err
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
