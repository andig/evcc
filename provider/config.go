package provider

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andig/evcc/util"
)

// Config is the general provider config
type Config struct {
	Type  string
	Other map[string]interface{} `mapstructure:",remain"`
}

// mqttConfig is the specific mqtt getter/setter configuration
type mqttConfig struct {
	Topic, Payload string // Payload only applies to setters
	Scale          float64
	Timeout        time.Duration
}

// MQTT singleton
var MQTT *MqttClient

func mqttFromConfig(other map[string]interface{}) (mqttConfig, error) {
	pc := mqttConfig{Scale: 1}
	if err := util.DecodeOther(other, &pc); err != nil {
		return pc, err
	}

	if MQTT == nil {
		return pc, errors.New("mqtt not configured")
	}

	return pc, nil
}

// NewFloatGetterFromConfig creates a FloatGetter from config
func NewFloatGetterFromConfig(config Config) (res func() (float64, error), err error) {
	switch strings.ToLower(config.Type) {
	case "calc":
		res, err = NewCalcFromConfig(config.Other)
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.FloatGetter
		}
	case "js":
		var prov *Javascript
		if prov, err = NewJavascriptProviderFromConfig(config.Other); err == nil {
			res = prov.FloatGetter
		}
	case "websocket", "ws":
		var prov *Socket
		if prov, err = NewSocketProviderFromConfig(config.Other); err == nil {
			res = prov.FloatGetter
		}
	case "mqtt":
		if pc, err := mqttFromConfig(config.Other); err == nil {
			res = MQTT.FloatGetter(pc.Topic, pc.Scale, pc.Timeout)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.FloatGetter
		}
	case "modbus":
		var prov *Modbus
		if prov, err = NewModbusFromConfig(config.Other); err == nil {
			res = prov.FloatGetter
		}
	default:
		return nil, fmt.Errorf("invalid plugin type: %s", config.Type)
	}

	return
}

// NewIntGetterFromConfig creates a IntGetter from config
func NewIntGetterFromConfig(config Config) (res func() (int64, error), err error) {
	switch strings.ToLower(config.Type) {
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.IntGetter
		}
	case "js":
		var prov *Javascript
		if prov, err = NewJavascriptProviderFromConfig(config.Other); err == nil {
			res = prov.IntGetter
		}
	case "websocket", "ws":
		var prov *Socket
		if prov, err = NewSocketProviderFromConfig(config.Other); err == nil {
			res = prov.IntGetter
		}
	case "mqtt":
		var pc mqttConfig
		if pc, err = mqttFromConfig(config.Other); err == nil {
			res = MQTT.IntGetter(pc.Topic, int64(pc.Scale), pc.Timeout)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.IntGetter
		}
	case "modbus":
		var prov *Modbus
		if prov, err = NewModbusFromConfig(config.Other); err == nil {
			res = prov.IntGetter
		}
	default:
		err = fmt.Errorf("invalid plugin type: %s", config.Type)
	}

	return
}

// NewStringGetterFromConfig creates a StringGetter from config
func NewStringGetterFromConfig(config Config) (res func() (string, error), err error) {
	switch strings.ToLower(config.Type) {
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.StringGetter
		}
	case "js":
		var prov *Javascript
		if prov, err = NewJavascriptProviderFromConfig(config.Other); err == nil {
			res = prov.StringGetter
		}
	case "websocket", "ws":
		var prov *Socket
		if prov, err = NewSocketProviderFromConfig(config.Other); err == nil {
			res = prov.StringGetter
		}
	case "mqtt":
		var pc mqttConfig
		if pc, err = mqttFromConfig(config.Other); err == nil {
			res = MQTT.StringGetter(pc.Topic, pc.Timeout)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.StringGetter
		}
	case "combined", "openwb":
		res, err = NewOpenWBStatusProviderFromConfig(config.Other)
	default:
		err = fmt.Errorf("invalid plugin type: %s", config.Type)
	}

	return
}

// NewBoolGetterFromConfig creates a BoolGetter from config
func NewBoolGetterFromConfig(config Config) (res func() (bool, error), err error) {
	switch strings.ToLower(config.Type) {
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.BoolGetter
		}
	case "js":
		var prov *Javascript
		if prov, err = NewJavascriptProviderFromConfig(config.Other); err == nil {
			res = prov.BoolGetter
		}
	case "websocket", "ws":
		var prov *Socket
		if prov, err = NewSocketProviderFromConfig(config.Other); err == nil {
			res = prov.BoolGetter
		}
	case "mqtt":
		var pc mqttConfig
		if pc, err = mqttFromConfig(config.Other); err == nil {
			res = MQTT.BoolGetter(pc.Topic, pc.Timeout)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.BoolGetter
		}
	default:
		err = fmt.Errorf("invalid plugin type: %s", config.Type)
	}

	return
}

// NewIntSetterFromConfig creates a IntSetter from config
func NewIntSetterFromConfig(param string, config Config) (res func(int64) error, err error) {
	switch strings.ToLower(config.Type) {
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.IntSetter
		}
	case "mqtt":
		var pc mqttConfig
		if pc, err = mqttFromConfig(config.Other); err == nil {
			res = MQTT.IntSetter(param, pc.Topic, pc.Payload)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.IntSetter(param)
		}
	default:
		err = fmt.Errorf("invalid setter type %s", config.Type)
	}

	return
}

// NewBoolSetterFromConfig creates a BoolSetter from config
func NewBoolSetterFromConfig(param string, config Config) (res func(bool) error, err error) {
	switch strings.ToLower(config.Type) {
	case "http":
		var prov *HTTP
		if prov, err = NewHTTPProviderFromConfig(config.Other); err == nil {
			res = prov.BoolSetter
		}
	case "mqtt":
		var pc mqttConfig
		if pc, err = mqttFromConfig(config.Other); err == nil {
			res = MQTT.BoolSetter(param, pc.Topic, pc.Payload)
		}
	case "script":
		var prov *Script
		if prov, err = NewScriptProviderFromConfig(config.Other); err == nil {
			res = prov.BoolSetter(param)
		}

	default:
		err = fmt.Errorf("invalid setter type %s", config.Type)
	}

	return
}
