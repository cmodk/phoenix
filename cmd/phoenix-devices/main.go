package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cmodk/go-simpleflake"
	"github.com/sirupsen/logrus"

	"github.com/cmodk/phoenix"
	phoenix_app "github.com/cmodk/phoenix/app"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

var (
	app                    = phoenix.New()
	lg                     = app.Logger
	debug                  = flag.Bool("debug", false, "Enable debug output")
	certificate_expiration = flag.String("certificate-expiration", "2160h", "Device certificate expiration time, default is 90 days = 2160 hours")

	certificate_expiration_time time.Duration
)

type deviceContextHandler func(http.ResponseWriter, *http.Request, *phoenix.Device)
type streamContextHandler func(http.ResponseWriter, *http.Request, *phoenix.Device, *phoenix.Stream)

func main() {
	flag.Parse()
	if *debug {
		app.Logger.Level = logrus.DebugLevel
	}

	var err error
	certificate_expiration_time, err = time.ParseDuration(*certificate_expiration)
	if err != nil {
		lg.WithField("error", err).Fatal("Error parsing certificate expiration string: %s\n", certificate_expiration)
	}

	if err := app.App.CheckAndUpdateDatabase(phoenix.DatabaseStructure); err != nil {
		panic(err)
	}

	app.Use(phoenix_app.Cors())

	app.Get("/info", infoHandler)

	app.Get("/device", deviceListHandler)
	app.Get("/device/{device}", deviceGetHandler)
	app.Post("/device/{device}/certificate", withParametricDevice(deviceCertificateRequestHandler))
	app.Get("/device/{device}/notification", withParametricDevice(deviceNotificationListHandler))
	app.Post("/device/{device}/notification", withParametricDevice(deviceNotificationPostHandler))
	app.Post("/device/{device}/notification/{notification}/override_stream_value", withParametricDevice(deviceNotificationOverrideStreamValueHandler))
	app.Get("/device/{device}/stream", withParametricDevice(deviceStreamListHandler))
	app.Get("/device/{device}/stream/{stream}", withParametricDevice(withParametricStream(deviceStreamValueListHandler)))
	app.Get("/device/{device}/sample", withParametricDevice(deviceSampleListHandler))
	app.Post("/device/{device}/command", withParametricDevice(deviceCommandCreateHandler))
	app.Get("/device/{device}/command/{command}", withParametricDevice(deviceCommandGetHandler))
	app.HandleEvent(phoenix.DeviceOnline{}, deviceOnline)

	app.LoadCertificates(true)
	app.Run()
}

func deviceOnline(event interface{}) error {

	log.Printf("Device online\n")

	return nil
}

func infoHandler(w http.ResponseWriter, r *http.Request) {

	info := struct {
		Version string `json:"version"`
	}{
		Version: phoenix.Version,
	}

	if err := json.NewEncoder(w).Encode(info); err != nil {
		app.HttpInternalError(w, err)
		return

	}

}

func deviceListHandler(w http.ResponseWriter, r *http.Request) {
	c := phoenix.DeviceCriteria{}
	if err := schema.NewDecoder().Decode(&c, r.URL.Query()); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	ds, err := app.Devices.List(c)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ds); err != nil {
		app.HttpInternalError(w, err)
		return
	}
}

func deviceGetHandler(w http.ResponseWriter, r *http.Request) {

	device_id := mux.Vars(r)["device"]

	d, err := app.Devices.Get(phoenix.DeviceCriteria{
		Guid: device_id,
	})
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(d); err != nil {
		app.HttpInternalError(w, err)
		return
	}
}

func deviceNotificationListHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

	c := phoenix.DeviceNotificationCriteria{
		From: time.Now().AddDate(0, 0, -1),
		To:   time.Now(),
	}

	if err := schema.NewDecoder().Decode(&c, r.URL.Query()); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	ns, err := d.NotificationList(c)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}
	app.JsonResponse(w, ns)
}

func deviceNotificationPostHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {
	auth_headers := r.Header["Authorization"]
	if len(auth_headers) == 0 {
		app.HttpUnauthorized(w, fmt.Errorf("Missing authentication"))
		return
	}

	auth_header := auth_headers[0]
	bearer := auth_header[7:]

	if d.Token == nil || bearer != *d.Token {
		app.HttpUnauthorized(w, fmt.Errorf("Invalid token for device"))
		return
	}

	if d.TokenExpiration == nil || d.TokenExpiration.Before(time.Now()) {
		err := fmt.Errorf("Token expired")
		app.Logger.WithField("device", d).WithField("error", err).Error(err)
		app.HttpUnauthorized(w, err)
		return
	}

	var n phoenix.DeviceNotification

	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	n.Id = simpleflake.Next()
	n.DeviceId = d.Id

	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now()
	}

	cmd := phoenix.DeviceNotificationCreate{
		Id:           n.Id,
		DeviceGuid:   d.Guid,
		Notification: n.Notification,
		Timestamp:    n.Timestamp,
		Parameters:   n.Parameters,
	}

	if err := app.Command.Create(cmd); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	//Return any pending commands to device
	commands, err := d.CommandsPending()
	if err != nil {
		lg.WithField("Error", err).Errorf("Error fetching pending commands for device: %s", d.Guid)
	}

	resp := struct {
		phoenix.DeviceNotification
		PendingCommands []phoenix.DeviceCommand `json:"pending_commands"`
	}{
		n,
		commands,
	}

	if err := app.JsonResponse(w, resp); err == nil {
		//Potential commands sent to device, mark them sent
		for _, cmd := range resp.PendingCommands {
			if err := d.CommandSent(&cmd); err != nil {
				lg.WithField("Error", err).Errorf("Error marking command sent")
			}
		}
	}

}

func deviceNotificationOverrideStreamValueHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

	notification_id := mux.Vars(r)["notification"]

	id, err := strconv.ParseUint(notification_id, 10, 64)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	/*
	 * 	Fetch override value and timestamp criteria
	 * Cannot put the id in the json payload, as the json parser uses float64 as type
	 */
	override := phoenix.StringMap{}

	if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	time_string, ok := override["timestamp"]
	if !ok {
		app.HttpBadRequest(w, fmt.Errorf("Missing timestamp in body"))
		return
	}

	timestamp, err := time.Parse(time.RFC3339, time_string.(string))
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	value, ok := override["override_value"]
	if !ok {
		app.HttpBadRequest(w, fmt.Errorf("Missing override value in body"))
		return
	}

	log.Printf("Searching for notification with id: %d\n", id)

	c := phoenix.DeviceNotificationCriteria{
		Id:        id,
		Timestamp: timestamp,
	}

	n, err := d.NotificationGet(c)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	par := phoenix.StringMap{}

	if err := json.Unmarshal(n.Parameters, &par); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	par["override_value"] = value

	n.Parameters, err = json.Marshal(par)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	if err := d.NotificationUpdateParameters(n); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	//Rewrite parameters again and trigger an update of the value and averages

	par["value"] = par["override_value"]
	delete(par, "override_value")

	n.Parameters, err = json.Marshal(par)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}
	app.Event.Publish(phoenix.DeviceNotificationCreated(*n))

	app.JsonResponse(w, n)

}

func withParametricDevice(h deviceContextHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		device_id := mux.Vars(r)["device"]
		if len(device_id) == 0 {
			app.HttpBadRequest(w, fmt.Errorf("Missing device id"))
			return
		}

		d, err := app.Devices.Get(phoenix.DeviceCriteria{
			Guid: device_id,
		})
		if err != nil {
			app.HttpBadRequest(w, fmt.Errorf("Device not found"))
			return
		}

		h(w, r, d)

	}
}

func withParametricStream(h streamContextHandler) deviceContextHandler {
	return func(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

		stream_code := mux.Vars(r)["stream"]
		if len(stream_code) == 0 {
			app.HttpBadRequest(w, fmt.Errorf("Missing stream code"))
			return
		}

		s, err := d.StreamGet(phoenix.StreamCriteria{
			Code: stream_code,
		})
		if err != nil {
			app.HttpBadRequest(w, fmt.Errorf("Stream not found"))
			return
		}

		h(w, r, d, s)

	}
}

func deviceStreamListHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {
	streams, err := d.StreamList(phoenix.StreamCriteria{})
	if err != nil {
		app.HttpInternalError(w, err)
		return
	}

	app.JsonResponse(w, streams)
}

func deviceSampleListHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {
	c := phoenix.SampleCriteria{
		From:      time.Now().UTC().AddDate(0, 0, -1),
		To:        time.Now(),
		Limit:     10000,
		Frequency: "hour",
	}

	if err := schema.NewDecoder().Decode(&c, r.URL.Query()); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	samples, err := d.SampleList(c)
	if err != nil {
		app.HttpInternalError(w, err)
		return
	}

	app.JsonResponse(w, samples)
}

func deviceStreamValueListHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device, s *phoenix.Stream) {
	c := phoenix.SampleCriteria{
		From:      time.Now().UTC().AddDate(0, 0, -1),
		To:        time.Now(),
		Limit:     10000,
		Frequency: "hour",
	}

	if err := schema.NewDecoder().Decode(&c, r.URL.Query()); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	c.Streams = []string{s.Code}

	samples, err := d.StreamValueList(c)
	if err != nil {
		app.HttpInternalError(w, err)
		return
	}

	app.JsonResponse(w, samples)
}

func deviceCommandCreateHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {
	var command phoenix.DeviceCommand

	if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	if err := d.CommandInsert(&command); err != nil {
		app.HttpInternalError(w, err)
		return
	}

	command.DeviceGuid = d.Guid

	if err := app.Event.Publish(phoenix.DeviceCommandCreated(command)); err != nil {
		app.HttpInternalError(w, err)
		return
	}

	app.JsonResponse(w, command)
}

func deviceCommandGetHandler(w http.ResponseWriter, r *http.Request, d *phoenix.Device) {

	command_id := mux.Vars(r)["command"]

	id, err := strconv.ParseUint(command_id, 10, 64)
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	command, err := d.CommandGet(
		phoenix.DeviceCommandCriteria{
			Id: id,
		})
	if err != nil {
		app.HttpBadRequest(w, err)
		return
	}

	app.JsonResponse(w, command)

}
