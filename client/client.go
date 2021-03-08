package client

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/cmodk/go-simplehttp"
	"github.com/cmodk/phoenix"
)

type Client struct {
	*simplehttp.SimpleHttp
}

func New(host string, logger *logrus.Logger) *Client {

	backend := simplehttp.New(host, logger)

	client := Client{&backend}

	return &client
}

func (client *Client) DeviceFind(device_id uint64) (*phoenix.Device, error) {

	url := fmt.Sprintf("/device?id=%d", device_id)

	data, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	var devices []phoenix.Device

	if err := json.Unmarshal([]byte(data), &devices); err != nil {
		return nil, err
	}

	if len(devices) != 1 {
		return nil, fmt.Errorf("Device not found %d", len(devices))
	}

	device := devices[0]

	return &device, nil

}
