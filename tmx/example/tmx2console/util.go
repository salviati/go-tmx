package main

import (
	"errors"
	"github.com/salviati/go-tmx/tmx"
)

var (
	PropertyNotUnique   = errors.New("Layer Property is not unique")
	PropertyUnavailable = errors.New("Property does not exist")
)

func GetProperty(p *tmx.Properties, name string) (value string, err error) {
	values := p.Get(name)
	if len(values) > 1 {
		err = PropertyNotUnique
		return
	}
	if len(value) == 0 {
		err = PropertyUnavailable
		return
	}
	value = values[0]
	return
}
