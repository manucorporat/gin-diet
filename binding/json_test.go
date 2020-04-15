// Copyright 2019 Gin Core Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"testing"

	"github.com/go-playground/assert"
)

func TestJSONBindingBindBody(t *testing.T) {
	var s struct {
		Foo string `json:"foo"`
	}
	err := jsonBinding{}.BindBody([]byte(`{"foo": "FOO"}`), &s)
	assert.Equal(t, err, nil)
	assert.Equal(t, "FOO", s.Foo)
}
