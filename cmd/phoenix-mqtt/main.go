package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/cmodk/go-mqtt"
	"github.com/cmodk/go-simpleflake"
	"github.com/cmodk/phoenix"
)

var (
	app = phoenix.New()
	lg  = app.Logger
	log = app.Logger

	mq *mqtt.Server

	no_tls = flag.Bool("disable-tls", false, "Disable tls")
	debug  = flag.Bool("debug", false, "Enable debug information")
)

func main() {
	flag.Parse()

	if *debug {
		app.Logger.Level = logrus.DebugLevel
	} else {
		app.Logger.Level = logrus.WarnLevel
	}

	if *no_tls == false {
		tls := NewTLSConfig()

		mq = mqtt.NewServer(tls)
	} else {
		mq = mqtt.NewServer(nil)
	}
	if err := mq.Subscribe("/device/+/sample", 2, SampleHandler); err != nil {
		panic(err)
	}

	if err := mq.Subscribe("/device/+/status", 2, StatusHandler); err != nil {
		panic(err)
	}

	if err := mq.Subscribe("/device/+/notification", 2, NotificationHandler); err != nil {
		panic(err)
	}

	if err := mq.Subscribe("/device/+/command/+", 2, CommandResponseHandler); err != nil {
		panic(err)
	}

	app.HandleEvent(phoenix.DeviceCommandCreated{}, deviceCommandCreated)

	go mq.Run()

	//Need seperate applications names for nsq
	application_name := filepath.Base(os.Args[0])
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	app.Event.SetListenName(application_name + "-" + hostname)

	go app.ListenEvents()

	app.Run()
}
func NewTLSConfig() *tls.Config {
	if err := app.LoadCertificates(false); err != nil {
		panic(err)
	}

	certpool := x509.NewCertPool()
	certpool.AddCert(app.CACertificate)

	for _, s := range certpool.Subjects() {
		log.Println(string(s))
	}

	// Import client certificate/key pair
	cert, err := tls.LoadX509KeyPair(app.CertificatePath+"/server.pem", app.CertificatePath+"/server.key.pem")
	if err != nil {
		panic(err)
	}

	// Just to print out the client certificate..
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}
	fmt.Println(cert.Leaf)

	// Create tls.Config with desired tls properties
	return &tls.Config{
		// RootCAs = certs used to verify server cert.
		RootCAs: certpool,
		// ClientAuth = whether to request cert from server.
		// Since the server is set up for SSL, this happens
		// anyways.
		ClientAuth: tls.RequireAndVerifyClientCert,
		// ClientCAs = certs used to validate client cert.
		ClientCAs: certpool,
		// InsecureSkipVerify = verify that cert contents
		// match server. IP matches what is in cert etc.
		InsecureSkipVerify: true,
		// Certificates = list of certs client sends to server.
		Certificates:          []tls.Certificate{cert},
		VerifyPeerCertificate: VerifyClient,
	}
}

func SampleHandler(s *mqtt.Server, msg mqtt.Message) error {
	var value float64
	var sb strings.Builder
	var stream string

	payload := msg.Payload

	if log.Level == logrus.DebugLevel {
		debugMessage := fmt.Sprintf("Payload(%d): ", len(payload))
		for _, v := range payload {
			debugMessage += fmt.Sprintf("'%c':0x%02x ", uint8(v), uint8(v))
		}
		log.Debug(debugMessage)
	}
	unix_time := binary.LittleEndian.Uint64(payload[0:8])
	value = Float64FromBytes(payload[8:16])

	for i := 16; i < len(payload); i++ {
		sb.WriteByte(payload[i])
		if payload[i] == 0x00 {
			break
		}
	}

	stream = sb.String()

	log.Debugf("TOPIC: %s\n", msg.Topic)
	log.Debugf("MSG: %s -> %d -> %f\n", stream, unix_time, value)

	//Get device id
	topic := strings.Split(msg.Topic, "/")
	device_id := topic[2]

	tm := time.Time{}
	if unix_time > 0 {
		//Need the milliseconds
		ms := int64(unix_time % 1000)

		s := int64(unix_time / 1000)
		tm = time.Unix(s, ms)
	} else {
		tm = time.Now()
	}

	log.Debugf("%s -> %s -> %s -> %f\n", device_id, stream, tm, value)

	raw_stream, err := json.Marshal(phoenix.Stream{
		Code:      stream,
		Timestamp: &tm,
		Value:     value,
	})
	if err != nil {
		return err
	}

	cmd := phoenix.DeviceNotificationCreate{
		Id:           simpleflake.Next(),
		DeviceGuid:   device_id,
		Notification: "stream",
		Timestamp:    time.Now().UTC(),
		Parameters:   json.RawMessage(raw_stream),
	}

	return app.Command.Create(cmd)

}

func StatusHandler(server *mqtt.Server, msg mqtt.Message) error {

	log.Printf("\n\n STATUS HANDLER \n\n")
	//Get device id
	topic := strings.Split(msg.Topic, "/")
	device_id := topic[2]

	//Find device
	d, err := app.Devices.Get(phoenix.DeviceCriteria{
		Guid: device_id,
	})
	if err != nil {
		lg.WithField("device_id", device_id).WithField("error", err).Error("Error looking up device")
		return err
	}

	status := string(msg.Payload)
	log.Printf("New device status: %s: %v\n", d.Guid, status)

	switch status {
	case "offline":
		if err := d.UpdateOnlineStatus(false); err != nil {
			return err
		}
	case "online":
		if err := d.UpdateOnlineStatus(true); err != nil {
			return err
		}
	default:
		lg.WithField("device_id", device_id).WithField("status", status).Error("Unknown status")
	}

	return nil

}

func NotificationHandler(server *mqtt.Server, msg mqtt.Message) error {
	fmt.Printf("TOPIC: %s\n", msg.Topic)
	fmt.Printf("MSG: %s\n", msg.Payload)

	//Get device id
	topic := strings.Split(msg.Topic, "/")
	fmt.Printf("topic: %v\n", topic)

	device_id := topic[2]

	log.Println(msg.Payload)
	n := phoenix.DeviceNotification{}
	if err := json.Unmarshal(msg.Payload, &n); err != nil {
		return err
	}

	log.Printf("Got notification from device: %s -> %v\n", device_id, n)

	cmd := phoenix.DeviceNotificationCreate{
		Id:           simpleflake.Next(),
		DeviceGuid:   device_id,
		Notification: n.Notification,
		Parameters:   n.Parameters,
	}

	if n.Timestamp.IsZero() {
		cmd.Timestamp = time.Now().UTC()
	} else {
		cmd.Timestamp = n.Timestamp
	}

	return app.Command.Create(cmd)
}

func CommandResponseHandler(s *mqtt.Server, msg mqtt.Message) error {

	log.Printf("Command response")
	payload := msg.Payload
	if log.Level == logrus.DebugLevel {
		debugMessage := fmt.Sprintf("Payload(%d): ", len(payload))
		for _, v := range payload {
			debugMessage += fmt.Sprintf("'%c':0x%02x ", uint8(v), uint8(v))
		}
		log.Debug(debugMessage)
	}

	topic := strings.Split(msg.Topic, "/")

	deviceGuid := topic[2]

	commandId, err := strconv.ParseUint(topic[4], 10, 64)
	if err != nil {
		return err
	}

	var response struct {
		Value interface{} `db:"value" json:"value"`
	}
	t := payload[0]
	switch t {
	case ConfigTypeDouble:
		response.Value = Float64FromBytes(payload[1:])
	default:
		return fmt.Errorf("Unhandled database type: %d", t)
	}

	log.Printf("Device: %s, Command: %d, Type: %d, Value: %v\n", deviceGuid, commandId, t, response)

	device, err := app.Devices.Get(phoenix.DeviceCriteria{Guid: deviceGuid})
	if err != nil {
		return err
	}

	device_command, err := device.CommandGet(phoenix.DeviceCommandCriteria{
		Id: commandId,
	})
	if err != nil {
		return err
	}

	log.Debugf("Updating command with response")
	return device.CommandResponse(device_command, response)

}

func Float64FromBytes(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

func Float64Bytes(float float64) []byte {
	bits := math.Float64bits(float)
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, bits)
	return bytes
}
