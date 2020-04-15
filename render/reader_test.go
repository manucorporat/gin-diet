// Copyright 2019 Gin Core Team.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package render

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/assert"
)

func TestReaderRenderNoHeaders(t *testing.T) {
	content := "test"
	r := Reader{
		ContentLength: int64(len(content)),
		Reader:        strings.NewReader(content),
	}
	err := r.Render(httptest.NewRecorder())
	assert.Equal(t, err, nil)
}
