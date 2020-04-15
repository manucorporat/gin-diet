// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"os"
	"testing"

	"github.com/go-playground/assert"
	"github.com/manucorporat/gin-diet/binding"
)

func init() {
	os.Setenv(EnvGinMode, TestMode)
}

func TestSetMode(t *testing.T) {
	assert.Equal(t, testCode, ginMode)
	assert.Equal(t, TestMode, Mode())
	os.Unsetenv(EnvGinMode)

	SetMode("")
	assert.Equal(t, debugCode, ginMode)
	assert.Equal(t, DebugMode, Mode())

	SetMode(DebugMode)
	assert.Equal(t, debugCode, ginMode)
	assert.Equal(t, DebugMode, Mode())

	SetMode(ReleaseMode)
	assert.Equal(t, releaseCode, ginMode)
	assert.Equal(t, ReleaseMode, Mode())

	SetMode(TestMode)
	assert.Equal(t, testCode, ginMode)
	assert.Equal(t, TestMode, Mode())

	Panics(t, func() { SetMode("unknown") })
}

func TestEnableJsonDecoderUseNumber(t *testing.T) {
	assert.Equal(t, false, binding.EnableDecoderUseNumber)
	EnableJsonDecoderUseNumber()
	assert.Equal(t, true, binding.EnableDecoderUseNumber)
}

func TestEnableJsonDecoderDisallowUnknownFields(t *testing.T) {
	assert.Equal(t, false, binding.EnableDecoderDisallowUnknownFields)
	EnableJsonDecoderDisallowUnknownFields()
	assert.Equal(t, true, binding.EnableDecoderDisallowUnknownFields)
}
