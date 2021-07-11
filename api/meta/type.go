package meta

import "reflect"

// Type describes a single type
type Type struct {
	Name   string       `json:"name"`
	Label  string       `json:"label"`
	Fields []Field      `json:"fields"`
	Type   reflect.Type `json:"-"`
}

// LoadType meta data from reflect.Type
func LoadType(t reflect.Type, name, label string) Type {
	if t.Kind() == reflect.Ptr {
		return LoadType(t.Elem(), name, label)
	}

	m := Type{
		Name:  name,
		Label: label,
		Type:  t,
	}

	if t.Kind() == reflect.Struct {
		m.Fields = handleFields(t)
	}
	return m
}

func handleFields(t reflect.Type) (fields []Field) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fields = append(fields, handleField(field)...)
	}
	return fields
}

func handleField(field reflect.StructField) (fields []Field) {
	// ignore unexported fields
	if field.PkgPath != "" {
		return
	}

	t := field.Type
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() == reflect.Struct {
		tagValues := getTagValues(field, "mapstructure")
		if _, ok := tagValues["squash"]; ok {
			return handleFields(t)
		}
	}

	fields = append(fields, LoadField(field))
	return
}
