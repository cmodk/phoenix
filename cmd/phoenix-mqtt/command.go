package main

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cmodk/phoenix"
)

const (
	CommandConfigRead = iota + 1
	CommandConfigWrite
	CommandSystemReboot = 10000
)

const (
	ConfigTypeString uint8 = iota
	ConfigTypeInt
	ConfigTypeDouble
)

var (
	DefaultQos     = 2
	deviceCommands = Commands{
		{"config_write", commandConfigWrite},
		{"config_read", commandConfigRead},
		{"reboot", commandReboot},
	}
)

type Command struct {
	Tag     string
	Handler func(*ConfigurationParameter) (*CommandPayload, error)
}

type Commands []Command

func (commands *Commands) GetCommand(tag string) (*Command, error) {
	for i, _ := range *commands {
		command := &((*commands)[i])
		if command.Tag == tag {
			return command, nil
		}

	}

	return nil, fmt.Errorf("Unknown command tag: %s\n", tag)
}

type CommandPayload struct {
	Id      uint64
	Tag     uint16
	Length  uint16
	Payload []uint8
	Qos     *int
}

func (cp *CommandPayload) ToBytes() []uint8 {
	bs := make([]byte, 12+cp.Length)

	log.Printf("ToBytes for %d with length %d\n", cp.Tag, cp.Length)
	bs[0] = uint8(cp.Id >> 56)
	bs[1] = uint8(cp.Id >> 48)
	bs[2] = uint8(cp.Id >> 40)
	bs[3] = uint8(cp.Id >> 32)
	bs[4] = uint8(cp.Id >> 24)
	bs[5] = uint8(cp.Id >> 16)
	bs[6] = uint8(cp.Id >> 8)
	bs[7] = uint8(cp.Id & 0xFF)

	bs[8] = uint8(cp.Tag >> 8)
	bs[9] = uint8(cp.Tag & 0xFF)

	bs[10] = uint8(cp.Length >> 8)
	bs[11] = uint8(cp.Length & 0xFF)

	for i := uint16(0); i < cp.Length; i++ {
		bs[12+i] = cp.Payload[i]
	}

	return bs
}

func deviceCommandCreated(event interface{}) error {
	e := event.(phoenix.DeviceCommandCreated)

	log.Debugf("E: %v\n", e)

	command, err := deviceCommands.GetCommand(e.Command)
	if err != nil {
		return err
	}

	var parameters *ConfigurationParameter
	if e.Parameters != nil {
		parameters, err = ParseConfigurationParameters(*e.Parameters)
		if err != nil {
			return err
		}
	}

	payload, err := command.Handler(parameters)
	if err != nil {
		return err
	}

	if payload.Qos == nil {
		payload.Qos = &DefaultQos
	}

	payload.Id = e.Id

	device_command_topic := fmt.Sprintf("/device/%s/command", e.DeviceGuid)
	log.Printf("Publishing command to %s\n", device_command_topic)
	if err := mq.Publish(device_command_topic, *payload.Qos, false, payload.ToBytes()); err != nil {
		return err
	}

	return nil

}

func commandConfigWrite(parameters *ConfigurationParameter) (*CommandPayload, error) {

	conf := []byte(*parameters.Configuration)
	conf_len := uint16(len(conf))

	value, value_len, err := parameters.ValuePayload()
	if err != nil {
		return nil, err
	}
	payload := CommandPayload{}
	payload.Tag = CommandConfigWrite
	payload.Length = uint16(5 + len(conf) + len(value))
	payload.Payload = make([]byte, payload.Length)

	switch *parameters.Type {
	case "string":
		payload.Payload[0] = ConfigTypeString
	case "double":
		payload.Payload[0] = ConfigTypeDouble
	default:
		return nil, fmt.Errorf("Unknown type for config_write: %s", parameters.Type)
	}
	payload.Payload[1] = uint8(conf_len >> 8)
	payload.Payload[2] = uint8(conf_len & 0xff)
	payload.Payload[3] = uint8(value_len >> 8)
	payload.Payload[4] = uint8(value_len & 0xff)

	payloadIndex := 5
	copy(payload.Payload[payloadIndex:], conf)
	copy(payload.Payload[payloadIndex+len(conf):], value)

	return &payload, nil

}

func commandConfigRead(parameters *ConfigurationParameter) (*CommandPayload, error) {

	conf := []byte(*parameters.Configuration)
	conf_len := uint16(len(conf))

	payload := CommandPayload{}
	payload.Tag = CommandConfigRead
	payload.Length = uint16(3 + len(conf))
	payload.Payload = make([]byte, payload.Length)

	if parameters.Type == nil {
		return nil, fmt.Errorf("Missing type for config_read")
	}

	configType := uint8(0)
	switch *parameters.Type {
	case "string":
		configType = ConfigTypeString
	case "double":
		configType = ConfigTypeDouble
	default:
		return nil, fmt.Errorf("Unknown type for config_read: %s", parameters.Type)
	}

	payload.Payload[0] = configType
	payload.Payload[1] = uint8(conf_len >> 8)
	payload.Payload[2] = uint8(conf_len & 0xff)

	copy(payload.Payload[3:], conf)

	return &payload, nil

}

func commandReboot(parameters *ConfigurationParameter) (*CommandPayload, error) {
	qos := 0
	payload := CommandPayload{
		Tag:    CommandSystemReboot,
		Length: 0,
		Qos:    &qos,
	}

	return &payload, nil
}

type ConfigurationParameter struct {
	Configuration *string     `json:"configuration"`
	Type          *string     `json:"type"`
	Value         interface{} `json:"value"`
}

func ParseConfigurationParameters(parameters json.RawMessage) (*ConfigurationParameter, error) {
	var config ConfigurationParameter

	if err := json.Unmarshal(parameters, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (cp *ConfigurationParameter) ValuePayload() ([]byte, uint16, error) {
	//	value := []byte(cp.Value)
	//	value_len := uint16(len(value))

	var value []byte
	var value_len uint16

	switch t := cp.Value.(type) {
	case string:
		value = []byte(t)
		value_len = uint16(len(t))
	case float64:
		value = Float64Bytes(t)
		value_len = 8
	default:
		return []byte{}, 0, fmt.Errorf("Unhandled configuration type: %s", reflect.TypeOf(t).String())
	}

	return value, value_len, nil
}
