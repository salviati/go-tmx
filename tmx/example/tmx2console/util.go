package main

import (
	"errors"
	"github.com/salviati/go-tmx/tmx"
)

var (
	PropertyNotUnique   = errors.New("Layer Property is not unique")
	PropertyUnavailable = errors.New("Property does not exist")
)

func GetProperty(properties []tmx.Property, name string) (value string, err error) {
	values := make([]string, 0)
	for i := range properties {
		if properties[i].Name == name {
			values = append(values, properties[i].Value)
		}
	}

	if len(values) > 1 {
		err = PropertyNotUnique
		return
	}

	if len(values) == 0 {
		err = PropertyUnavailable
		return
	}

	value = values[0]
	return
}
