package sma

import (
	"fmt"
	"sync"
	"time"

	"github.com/andig/evcc/util"
	"github.com/imdario/mergo"
	"gitlab.com/bboehmke/sunny"
)

// Device holds information for a Device and provides interface to get values
type Device struct {
	*sunny.Device

	log    *util.Logger
	mux    *util.Waiter
	values map[sunny.ValueID]interface{}
	once   sync.Once
}

// StartUpdateLoop if not already started
func (d *Device) StartUpdateLoop() {
	d.once.Do(func() {
		go func() {
			d.updateValues()
			for range time.NewTicker(time.Second * 5).C {
				d.updateValues()
			}
		}()
	})
}

func (d *Device) updateValues() {
	d.mux.Lock()
	defer d.mux.Unlock()

	values, err := d.Device.GetValues()
	if err == nil {
		err = mergo.Merge(&d.values, values, mergo.WithOverride)
		d.mux.Update()
	}

	if err != nil {
		d.log.ERROR.Println(err)
	}
}

func (d *Device) Values() (map[sunny.ValueID]interface{}, error) {
	// ensure update loop was started
	d.StartUpdateLoop()

	elapsed := d.mux.LockWithTimeout()
	defer d.mux.Unlock()

	if elapsed > 0 {
		return nil, fmt.Errorf("update timeout: %v", elapsed.Truncate(time.Second))
	}

	// return a copy of the map to avoid race conditions
	values := make(map[sunny.ValueID]interface{}, len(d.values))
	for key, value := range d.values {
		values[key] = value
	}
	return values, nil
}

func AsFloat(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case nil:
		return 0
	default:
		util.NewLogger("sma").WARN.Printf("unknown value type: %T", value)
		return 0
	}
}
