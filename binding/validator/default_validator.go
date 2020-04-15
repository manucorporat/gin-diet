// Copyright 2017 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package validator

import (
	"reflect"
	"sync"

	"github.com/go-playground/validator/v10"
)

// DefaultValidator uses go-playground/validator/v10 under the hood
type DefaultValidator struct {
	once     sync.Once
	validate *validator.Validate
}

// New creates a new validator usable by gin bindings
func New() *DefaultValidator {
	return &DefaultValidator{}
}

// ValidateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *DefaultValidator) ValidateStruct(obj interface{}) error {
	value := reflect.ValueOf(obj)
	valueType := value.Kind()
	if valueType == reflect.Ptr {
		valueType = value.Elem().Kind()
	}
	if valueType == reflect.Struct {
		v.lazyinit()
		if err := v.validate.Struct(obj); err != nil {
			return err
		}
	}
	return nil
}

// Engine returns the underlying validator engine which powers the default
// Validator instance. This is useful if you want to register custom validations
// or struct level validations. See validator GoDoc for more info -
// https://godoc.org/gopkg.in/go-playground/validator.v8
func (v *DefaultValidator) Engine() interface{} {
	v.lazyinit()
	return v.validate
}

func (v *DefaultValidator) lazyinit() {
	v.once.Do(func() {
		v.validate = validator.New()
		v.validate.SetTagName("binding")
	})
}
