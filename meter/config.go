package meter

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/api/meta"
	"github.com/andig/evcc/util"
)

type meterRegistry map[string]meta.Type

func (r meterRegistry) Add(name string, label string, meter api.Meter) {
	if _, exists := r[name]; exists {
		panic(fmt.Sprintf("cannot register duplicate meter type: %s", name))
	}

	r[name] = meta.LoadType(reflect.TypeOf(meter), name, label)
}

func (r meterRegistry) Get(name string) (meta.Type, error) {
	t, exists := r[name]
	if !exists {
		return meta.Type{}, fmt.Errorf("meter type not registered: %s", name)
	}
	return t, nil
}

var registry meterRegistry = make(map[string]meta.Type)

// NewFromConfig creates meter from configuration
func NewFromConfig(typ string, other map[string]interface{}) (api.Meter, error) {
	d, err := registry.Get(strings.ToLower(typ))
	if err != nil {
		return nil, fmt.Errorf("invalid meter type: %s", typ)
	}

	v := reflect.New(d.Type).Interface().(api.Meter)

	if err := util.DecodeOther(other, v); err != nil {
		return nil, err
	}

	return v, nil
}

// Types exports the public configuration types
func Types() (types []meta.Type) {
	for _, typ := range registry {
		types = append(types, typ)
	}
	return types
}
