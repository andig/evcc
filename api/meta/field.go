package meta

import (
	"reflect"
	"strings"
	"time"
)

const (
	typeDuration = "duration"
	typePlugin   = "plugin"
	typePassword = "password"
	typeText     = "text"
)

// Field is the meta data format for the type description
type Field struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Required bool          `json:"required,omitempty"`
	Hidden   bool          `json:"hidden,omitempty"`
	Label    string        `json:"label,omitempty"`
	Unit     string        `json:"unit,omitempty"`
	Enum     []interface{} `json:"enum,omitempty"`
	Default  interface{}   `json:"default,omitempty"`
	Children []Field       `json:"children,omitempty"`
}

// LoadField meta data from struct field
func LoadField(field reflect.StructField) Field {
	f := Field{
		Name:  field.Name,
		Label: field.Name,
	}

	// meta data from validation
	validateTags := getTagValues(field, "validate")
	_, f.Required = validateTags["required"]
	if oneof, ok := validateTags["oneof"]; ok {
		for _, val := range strings.Split(oneof, " ") {
			f.Enum = append(f.Enum, val)
		}
	}

	// additional meta data
	metaTags := getTagValues(field, "meta")
	_, f.Hidden = metaTags["hide"]
	if unit, ok := metaTags["unit"]; ok {
		f.Unit = unit
	}

	if defValue, ok := field.Tag.Lookup("default"); ok {
		f.Default = defValue
	}

	if label, ok := field.Tag.Lookup("label"); ok {
		f.Label = label
	}

	t := field.Type
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// simplify value type
	if t.Kind() == reflect.Slice {
		f.Type = "[]" + simplifyType(t, metaTags)
	} else {
		f.Type = simplifyType(t, metaTags)
	}

	if t.Kind() == reflect.Struct && t.String() != "provider.Config" {
		f.Children = handleFields(t)
	}
	return f
}

// simplifyType of field
func simplifyType(t reflect.Type, meta map[string]string) string {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	if t == reflect.TypeOf(time.Duration(0)) {
		return typeDuration
	}

	if t.Kind() == reflect.Struct && t.String() == "provider.Config" {
		return typePlugin
	}

	if _, ok := meta["secret"]; ok {
		return typePassword
	}

	if _, ok := meta["text"]; ok {
		return typeText
	}

	return t.String()
}

// getTagValues returns a map with all tags and optional values
func getTagValues(field reflect.StructField, tag string) map[string]string {
	m := make(map[string]string)

	tagValues, ok := field.Tag.Lookup(tag)
	if !ok {
		return m
	}

	for _, value := range strings.Split(tagValues, ",") {
		splits := strings.SplitN(value, "=", 2)
		if len(splits) > 1 {
			m[splits[0]] = splits[1]
		} else {
			m[splits[0]] = ""
		}
	}

	return m
}
