package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/cmodk/go-simplehttp"
	"github.com/cmodk/phoenix"
	"github.com/cmodk/phoenix/app"
	"github.com/gocql/gocql"
)

type PhoenixCommand struct {
	Command    func() error
	RequireApp bool
}

var (
	ph  *phoenix.Phoenix
	lg  = logrus.New()
	log = lg

	remote_host  = flag.String("remote-host", "localhost", "Remote host adddress")
	device_guid  = flag.String("device", "", "Device guid")
	device_token = flag.String("device-token", "", "Device token")
	from_arg     = flag.String("from", "", "Optional range for data selection")
	to_arg       = flag.String("to", "", "Optional range for data selection")
	stream       = flag.String("stream", "", "Stream code")
	frequency    = flag.String("frequency", "", "Average frequency")

	noapp         bool
	debug         bool
	command_arg   string
	event_arg     string
	device_id_arg uint64
	nsq_host_arg  string
	namespace_arg string
	kubeconfig    string
	podname_arg   string
	keyspace_arg  string
	commands      = map[string]PhoenixCommand{
		//"generate-mqtt-user": generateMQTTUserCertificate,
		"loop":                                      PhoenixCommand{helperLoop, false},
		"recreate-pods":                             PhoenixCommand{k8sRecreatePods, false},
		"cassandra-create-keyspace":                 PhoenixCommand{cassandraCreateKeyspace, false},
		"cassandra-create-notification-table":       PhoenixCommand{cassandraCreateNotificationTable, true},
		"cassandra-create-sample-table":             PhoenixCommand{cassandraCreateSampleTable, true},
		"cassandra-create-sample-aggregated-tables": PhoenixCommand{cassandraCreateSampleAggregatedTables, true},
		"cassandra-create-stream-string-table":      PhoenixCommand{cassandraCreateStreamStringTable, true},
		"docker-build-images":                       PhoenixCommand{dockerBuildImages, false},
		"device-migrate-data":                       PhoenixCommand{deviceMigrateData, true},
		"device-samples-schedule-average":           PhoenixCommand{deviceSampleScheduleAverage, true},
		"device-stream-string-reupdate":             PhoenixCommand{deviceStreamStringReUpdate, true},
	}
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&namespace_arg, "namespace", "", "K8s namespace")
	flag.StringVar(&podname_arg, "pod", "", "K8s pod name")
	flag.StringVar(&event_arg, "event", "", "Event")
	flag.Uint64Var(&device_id_arg, "device-id", 0, "Device id")
	flag.StringVar(&command_arg, "cmd", "loop", "Command to fire")
	flag.StringVar(&nsq_host_arg, "nsq-host", "127.0.0.1:4151", "Nsq host to connect to")
	flag.BoolVar(&noapp, "noapp", false, "Do not create the application")
	flag.BoolVar(&debug, "debug", false, "Enable debug")
	flag.Parse()

	app.CheckFlags()

	cmd, ok := commands[command_arg]
	if !ok {
		log.Printf("Unknown command: %s\n", command_arg)
		printCommands()
		return
	}

	if cmd.RequireApp {
		ph = phoenix.New()
		lg = ph.Logger

		//phoenix.SetDebug(debug)
	}

	if debug {
		lg.Level = logrus.DebugLevel
	}

	if err := cmd.Command(); err != nil {
		panic(err)
	}

}

func printCommands() {
	log.Printf("Available commands:")
	for cmd, _ := range commands {
		log.Println(cmd)
	}
}

func helperLoop() error {
	log.Printf("No command specified, loop for the sake of kubernetes\n")

	for {
		time.Sleep(time.Second * 60)
	}

	return nil
}

func cassandraCreateKeyspace() error {
	env := os.Getenv("PHOENIX_ENV")
	if env == "" {
		env = "dev"
	}

	log.Printf("Running in environment: %s\n", env)

	config, err := app.LoadConfig(env)
	if err != nil {
		return err
	}

	cluster := gocql.NewCluster(config.Cassandra.Nodes)
	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	keyspace := "phoenix"
	replication := 1

	err = session.Query(fmt.Sprintf(`CREATE KEYSPACE %s
    WITH replication = {
              'class' : 'SimpleStrategy',
	              'replication_factor' : %d
		          }`, keyspace, replication)).Exec()
	if err != nil {
		return err
	}

	if err := cassandraCreateNotificationTable(); err != nil {
		return err
	}

	if err := cassandraCreateSampleTable(); err != nil {
		return err
	}

	if err := cassandraCreateStreamStringTable(); err != nil {
		return err
	}

	return nil
}

func cassandraCreateNotificationTable() error {
	err := ph.Cassandra.Query(`
CREATE TABLE notifications (
     id BIGINT,
     device VARCHAR,
     timestamp TIMESTAMP,
     notification VARCHAR,
     parameters TEXT,
     PRIMARY KEY ( device, timestamp, id)
) WITH CLUSTERING ORDER BY ( timestamp DESC);
`).Exec()
	if err != nil {
		return err
	}
	err = ph.Cassandra.Query(`
CREATE INDEX n_notification_index ON notifications(notification);
`).Exec()
	if err != nil {
		return err
	}
	err = ph.Cassandra.Query(`
CREATE INDEX n_id_index ON notifications(id);
`).Exec()
	if err != nil {
		return err
	}

	return nil
}

func cassandraCreateSampleTable() error {
	return ph.Cassandra.Query(`
CREATE TABLE samples(
    device text,
    stream text,
    timestamp timestamp,
    value double,
    PRIMARY KEY ((device, stream), timestamp)
) WITH CLUSTERING ORDER BY (timestamp DESC)
`).Exec()

}

func cassandraCreateSampleAggregatedTables() error {
	keys := []string{
		"minute",
		"hour",
		"day",
	}

	for _, key := range keys {
		query := ph.Cassandra.Query(fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS samples_%s(
    device text,
    stream text,
    timestamp timestamp,
    average double,
    max double,
    min double,
    count int,
    PRIMARY KEY ((device, stream), timestamp)
) WITH CLUSTERING ORDER BY (timestamp DESC)
`, key))

		log.Printf("Executing %s\n", query.String())
		if err := query.Exec(); err != nil {
			return err
		}
	}

	return nil

}

func cassandraCreateStreamStringTable() error {
	return ph.Cassandra.Query(`
CREATE TABLE stream_strings(
    device text,
    stream text,
    timestamp timestamp,
    value text,
    PRIMARY KEY ((device, stream), timestamp)
) WITH CLUSTERING ORDER BY (timestamp DESC)
`).Exec()

}

func deviceMigrateData() error {

	hh := simplehttp.New(*remote_host, lg)
	hh.SetBearerAuth(*device_token)

	url := fmt.Sprintf("/device/%s/notification", *device_guid)

	from := time.Unix(0, 0).UTC()
	to := time.Now().UTC()

	if len(*to_arg) > 0 {
		var err error
		to, err = time.Parse(time.RFC3339, *to_arg)
		if err != nil {
			return err
		}
	}

	query := ph.Cassandra.Query("SELECT * FROM notifications WHERE device = ? and timestamp > ? and timestamp < ?",
		*device_guid,
		from,
		to)

	log.Printf("Executing %s\n", query.String())
	i := 0
	iter := query.Iter()
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		i++
		notification := struct {
			Notification string          `json:"notification"`
			Timestamp    time.Time       `json:"timestamp"`
			Parameters   json.RawMessage `json:"parameters"`
		}{
			Notification: row["notification"].(string),
			Timestamp:    row["timestamp"].(time.Time),
			Parameters:   json.RawMessage(row["parameters"].(string)),
		}

		data, err := json.Marshal(notification)
		if err != nil {
			return err
		}

		log.Printf("%d: %v\n", i, string(data))

		resp, err := hh.Post(url, string(data))
		log.Printf("Resp: %s\n", string(resp))
		if err != nil {
			return err
		}

	}

	if err := iter.Close(); err != nil {
		lg.WithField("Error", err).Errorf("Error querying scylla")
		return err
	}
	return nil
}

func deviceSampleScheduleAverage() error {
	ctx := context.Background()

	from, err := time.Parse(time.RFC3339, *from_arg)
	if err != nil {
		return err
	}

	to := time.Now()

	if len(*to_arg) > 0 {
		var err error
		to, err = time.Parse(time.RFC3339, *to_arg)
		if err != nil {
			return err
		}
	}

	d, err := ph.Devices.Get(phoenix.DeviceCriteria{
		Guid: *device_guid,
	})
	if err != nil {
		log.Error("Device not found")
		return err
	}

	streams, err := d.StreamList(phoenix.StreamCriteria{})
	if err != nil {
		return err
	}

	stream_exists := false
	for _, s := range *streams {
		if s.Code == *stream {
			stream_exists = true
			break
		}
	}

	if stream_exists == false {
		return fmt.Errorf("Stream does not exists for device")
	}

	current := from

	for average_key, average_config := range phoenix.AverageConfigs {
		if average_key != "minute" {
			for current.Before(to) {
				phoenix.ScheduleCalculation(ph.Redis, ctx, current, average_key, *device_guid, *stream)
			}

			current = current.Add(average_config.Duration)
		}
	}

	return nil
}

func deviceStreamStringReUpdate() error {

	c := phoenix.DeviceCriteria{}

	if len(*device_guid) > 0 {
		c.Guid = *device_guid
	}
	devices, err := ph.Devices.List(c)
	if err != nil {
		log.Error("Could not get devices")
		return err
	}

	for _, d := range *devices {

		log.Printf("Processing device: %s", d.Guid)
		//		current, err := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
		current, err := time.Parse(time.RFC3339, "2021-05-18T00:00:00Z")
		if err != nil {
			return err
		}

		for current.Before(time.Now()) {
			to := current.Add(time.Hour)
			notifications, err := d.NotificationList(phoenix.DeviceNotificationCriteria{
				From: current,
				To:   to,
			})
			if err != nil {
				return err
			}

			log.Debugf("%s: Got %d notifications", current.Format(time.RFC3339), len(notifications))

			for _, n := range notifications {
				if n.Notification == "stream" {
					var s phoenix.Stream

					if n.Parameters[0] == 91 {
						//This is an array, which must be an error from testing the first batch stream notification
						log.Warningf("%s: Ignoring notification with bad stream parameters, seems to be an array", d.Guid)
						continue
					}

					if err := json.Unmarshal(n.Parameters, &s); err != nil {
						log.WithField("notification", n).WithField("parameters", string(n.Parameters)).WithField("stream", s).Error(err)
						continue
					}
					value, ok := s.Value.(string)
					if ok {
						timestamp := s.Timestamp
						if timestamp == nil || timestamp.IsZero() {
							//Use notification time
							timestamp = &(n.Timestamp)
						}
						log.Debugf("String: %s -> %s -> %s", s.Code, timestamp, value)
						update := phoenix.StreamUpdated(s)
						update.DeviceGuid = &(d.Guid)
						update.DeviceId = d.Id
						phoenix.StringSave(update)
					}
				}
			}

			current = to
		}
	}

	return nil

}
