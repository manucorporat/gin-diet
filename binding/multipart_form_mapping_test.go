// Copyright 2019 Gin Core Team.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/go-playground/assert"
)

func TestFormMultipartBindingBindOneFile(t *testing.T) {
	var s struct {
		FileValue   multipart.FileHeader     `form:"file"`
		FilePtr     *multipart.FileHeader    `form:"file"`
		SliceValues []multipart.FileHeader   `form:"file"`
		SlicePtrs   []*multipart.FileHeader  `form:"file"`
		ArrayValues [1]multipart.FileHeader  `form:"file"`
		ArrayPtrs   [1]*multipart.FileHeader `form:"file"`
	}
	file := testFile{"file", "file1", []byte("hello")}

	req := createRequestMultipartFiles(t, file)
	err := FormMultipart.Bind(req, &s)
	assert.Equal(t, nil, err)

	assertMultipartFileHeader(t, &s.FileValue, file)
	assertMultipartFileHeader(t, s.FilePtr, file)
	assert.Equal(t, len(s.SliceValues), 1)
	assertMultipartFileHeader(t, &s.SliceValues[0], file)
	assert.Equal(t, len(s.SlicePtrs), 1)
	assertMultipartFileHeader(t, s.SlicePtrs[0], file)
	assertMultipartFileHeader(t, &s.ArrayValues[0], file)
	assertMultipartFileHeader(t, s.ArrayPtrs[0], file)
}

func TestFormMultipartBindingBindTwoFiles(t *testing.T) {
	var s struct {
		SliceValues []multipart.FileHeader   `form:"file"`
		SlicePtrs   []*multipart.FileHeader  `form:"file"`
		ArrayValues [2]multipart.FileHeader  `form:"file"`
		ArrayPtrs   [2]*multipart.FileHeader `form:"file"`
	}
	files := []testFile{
		{"file", "file1", []byte("hello")},
		{"file", "file2", []byte("world")},
	}

	req := createRequestMultipartFiles(t, files...)
	err := FormMultipart.Bind(req, &s)
	assert.Equal(t, nil, err)

	assert.Equal(t, len(s.SliceValues), len(files))
	assert.Equal(t, len(s.SlicePtrs), len(files))
	assert.Equal(t, len(s.ArrayValues), len(files))
	assert.Equal(t, len(s.ArrayPtrs), len(files))

	for i, file := range files {
		assertMultipartFileHeader(t, &s.SliceValues[i], file)
		assertMultipartFileHeader(t, s.SlicePtrs[i], file)
		assertMultipartFileHeader(t, &s.ArrayValues[i], file)
		assertMultipartFileHeader(t, s.ArrayPtrs[i], file)
	}
}

func TestFormMultipartBindingBindError(t *testing.T) {
	files := []testFile{
		{"file", "file1", []byte("hello")},
		{"file", "file2", []byte("world")},
	}

	for _, tt := range []struct {
		name string
		s    interface{}
	}{
		{"wrong type", &struct {
			Files int `form:"file"`
		}{}},
		{"wrong array size", &struct {
			Files [1]*multipart.FileHeader `form:"file"`
		}{}},
		{"wrong slice type", &struct {
			Files []int `form:"file"`
		}{}},
	} {
		req := createRequestMultipartFiles(t, files...)
		err := FormMultipart.Bind(req, tt.s)
		assert.NotEqual(t, nil, err)
	}
}

type testFile struct {
	Fieldname string
	Filename  string
	Content   []byte
}

func createRequestMultipartFiles(t *testing.T, files ...testFile) *http.Request {
	var body bytes.Buffer

	mw := multipart.NewWriter(&body)
	for _, file := range files {
		fw, err := mw.CreateFormFile(file.Fieldname, file.Filename)
		assert.Equal(t, nil, err)

		n, err := fw.Write(file.Content)
		assert.Equal(t, nil, err)
		assert.Equal(t, len(file.Content), n)
	}
	err := mw.Close()
	assert.Equal(t, nil, err)

	req, err := http.NewRequest("POST", "/", &body)
	assert.Equal(t, nil, err)

	req.Header.Set("Content-Type", MIMEMultipartPOSTForm+"; boundary="+mw.Boundary())
	return req
}

func assertMultipartFileHeader(t *testing.T, fh *multipart.FileHeader, file testFile) {
	assert.Equal(t, file.Filename, fh.Filename)
	// assert.Equal(t, int64(len(file.Content)), fh.Size) // fh.Size does not exist on go1.8

	fl, err := fh.Open()
	assert.Equal(t, nil, err)

	body, err := ioutil.ReadAll(fl)
	assert.Equal(t, nil, err)
	assert.Equal(t, string(file.Content), string(body))

	err = fl.Close()
	assert.Equal(t, nil, err)
}
