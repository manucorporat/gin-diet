// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/go-playground/assert"
)

// TODO
// func debugRoute(httpMethod, absolutePath string, handlers HandlersChain) {
// func debugPrint(format string, values ...interface{}) {

func TestIsDebugging(t *testing.T) {
	SetMode(DebugMode)
	assert.Equal(t, true, IsDebugging())
	SetMode(ReleaseMode)
	assert.Equal(t, false, IsDebugging())
	SetMode(TestMode)
	assert.Equal(t, false, IsDebugging())
}

func TestDebugPrint(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		SetMode(ReleaseMode)
		debugPrint("DEBUG this!")
		SetMode(TestMode)
		debugPrint("DEBUG this!")
		SetMode(DebugMode)
		debugPrint("these are %d %s", 2, "error messages")
		SetMode(TestMode)
	})
	assert.Equal(t, "[GIN-debug] these are 2 error messages\n", re)
}

func TestDebugPrintError(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		debugPrintError(nil)
		debugPrintError(errors.New("this is an error"))
		SetMode(TestMode)
	})
	assert.Equal(t, "[GIN-debug] [ERROR] this is an error\n", re)
}

func TestDebugPrintRoutes(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		debugPrintRoute("GET", "/path/to/route/:param", HandlersChain{func(c *Context) {}, handlerNameTest})
		SetMode(TestMode)
	})
	assert.MatchRegex(t, re, `^\[GIN-debug\] GET    /path/to/route/:param     --> (.*/vendor/)?github.com/manucorporat/gin-diet.handlerNameTest \(2 handlers\)\n$`)
}

func TestDebugPrintLoadTemplate(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		templ := template.Must(template.New("").Delims("{[{", "}]}").ParseGlob("./testdata/template/hello.tmpl"))
		debugPrintLoadTemplate(templ)
		SetMode(TestMode)
	})
	assert.MatchRegex(t, re, `^\[GIN-debug\] Loaded HTML Templates \(2\): \n(\t- \n|\t- hello\.tmpl\n){2}\n`)
}

func TestDebugPrintWARNINGSetHTMLTemplate(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		debugPrintWARNINGSetHTMLTemplate()
		SetMode(TestMode)
	})
	assert.Equal(t, "[GIN-debug] [WARNING] Since SetHTMLTemplate() is NOT thread-safe. It should only be called\nat initialization. ie. before any route is registered or the router is listening in a socket:\n\n\trouter := gin.Default()\n\trouter.SetHTMLTemplate(template) // << good place\n\n", re)
}

func TestDebugPrintWARNINGDefault(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		debugPrintWARNINGDefault()
		SetMode(TestMode)
	})
	m, e := getMinVer(runtime.Version())
	if e == nil && m <= ginSupportMinGoVer {
		assert.Equal(t, "[GIN-debug] [WARNING] Now Gin requires Go 1.11 or later and Go 1.12 will be required soon.\n\n[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.\n\n", re)
	} else {
		assert.Equal(t, "[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.\n\n", re)
	}
}

func TestDebugPrintWARNINGNew(t *testing.T) {
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		debugPrintWARNINGNew()
		SetMode(TestMode)
	})
	assert.Equal(t, "[GIN-debug] [WARNING] Running in \"debug\" mode. Switch to \"release\" mode in production.\n - using env:\texport GIN_MODE=release\n - using code:\tgin.SetMode(gin.ReleaseMode)\n\n", re)
}

func captureOutput(t *testing.T, f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	defaultWriter := DefaultWriter
	defaultErrorWriter := DefaultErrorWriter
	defer func() {
		DefaultWriter = defaultWriter
		DefaultErrorWriter = defaultErrorWriter
		log.SetOutput(os.Stderr)
	}()
	DefaultWriter = writer
	DefaultErrorWriter = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		_, err := io.Copy(&buf, reader)
		assert.Equal(t, err, nil)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func TestGetMinVer(t *testing.T) {
	var m uint64
	var e error
	_, e = getMinVer("go1")
	assert.NotEqual(t, nil, e)
	m, e = getMinVer("go1.1")
	assert.Equal(t, uint64(1), m)
	assert.Equal(t, nil, e)
	m, e = getMinVer("go1.1.1")
	assert.Equal(t, nil, e)
	assert.Equal(t, uint64(1), m)
	_, e = getMinVer("go1.1.1.1")
	assert.NotEqual(t, nil, e)
}
