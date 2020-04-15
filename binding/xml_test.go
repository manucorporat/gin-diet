// Copyright 2019 Gin Core Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"testing"

	"github.com/go-playground/assert"
)

func TestXMLBindingBindBody(t *testing.T) {
	var s struct {
		Foo string `xml:"foo"`
	}
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<root>
   <foo>FOO</foo>
</root>`
	err := xmlBinding{}.BindBody([]byte(xmlBody), &s)
	assert.Equal(t, err, nil)
	assert.Equal(t, "FOO", s.Foo)
}
