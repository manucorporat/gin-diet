// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/assert"
	"github.com/manucorporat/gin-diet/binding"
)

var _ context.Context = &Context{}

// Unit tests TODO
// func (c *Context) File(filepath string) {
// func (c *Context) Negotiate(code int, config Negotiate) {
// BAD case: func (c *Context) Render(code int, render render.Render, obj ...interface{}) {
// test that information is not leaked when reusing Contexts (using the Pool)

func createMultipartRequest() *http.Request {
	boundary := "--testboundary"
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	defer mw.Close()

	must(mw.SetBoundary(boundary))
	must(mw.WriteField("foo", "bar"))
	must(mw.WriteField("bar", "10"))
	must(mw.WriteField("bar", "foo2"))
	must(mw.WriteField("array", "first"))
	must(mw.WriteField("array", "second"))
	must(mw.WriteField("id", ""))
	must(mw.WriteField("time_local", "31/12/2016 14:55"))
	must(mw.WriteField("time_utc", "31/12/2016 14:55"))
	must(mw.WriteField("time_location", "31/12/2016 14:55"))
	must(mw.WriteField("names[a]", "thinkerou"))
	must(mw.WriteField("names[b]", "tianou"))
	req, err := http.NewRequest("POST", "/", body)
	must(err)
	req.Header.Set("Content-Type", MIMEMultipartPOSTForm+"; boundary="+boundary)
	return req
}

func must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func TestContextFormFile(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	w, err := mw.CreateFormFile("file", "test")
	assert.Equal(t, err, nil)
	_, err = w.Write([]byte("test"))
	assert.Equal(t, err, nil)

	mw.Close()
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	f, err := c.FormFile("file")
	assert.Equal(t, err, nil)
	assert.Equal(t, "test", f.Filename)

	assert.Equal(t, c.SaveUploadedFile(f, "test"), nil)
}

func TestContextFormFileFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	mw.Close()
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.engine.MaxMultipartMemory = 8 << 20
	f, err := c.FormFile("file")
	assert.NotEqual(t, nil, err)
	assert.Equal(t, f, nil)
}

func TestContextMultipartForm(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	assert.Equal(t, mw.WriteField("foo", "bar"), nil)
	w, err := mw.CreateFormFile("file", "test")
	assert.Equal(t, err, nil)
	_, err = w.Write([]byte("test"))
	assert.Equal(t, err, nil)

	mw.Close()
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	f, err := c.MultipartForm()
	assert.Equal(t, err, nil)
	assert.NotEqual(t, f, nil)

	assert.Equal(t, c.SaveUploadedFile(f.File["file"][0], "test"), nil)
}

func TestSaveUploadedOpenFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	mw.Close()

	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())

	f := &multipart.FileHeader{
		Filename: "file",
	}
	assert.NotEqual(t, c.SaveUploadedFile(f, "test"), nil)
}

func TestSaveUploadedCreateFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	w, err := mw.CreateFormFile("file", "test")
	assert.Equal(t, err, nil)
	_, err = w.Write([]byte("test"))
	assert.Equal(t, err, nil)
	mw.Close()
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	f, err := c.FormFile("file")
	assert.Equal(t, err, nil)
	assert.Equal(t, "test", f.Filename)

	assert.NotEqual(t, c.SaveUploadedFile(f, "/"), nil)
}

func TestContextReset(t *testing.T) {
	router := New()
	c := router.allocateContext()
	assert.Equal(t, c.engine == router, true)

	c.index = 2
	c.Writer = &responseWriter{ResponseWriter: httptest.NewRecorder()}
	c.Params = Params{Param{}}
	c.Error(errors.New("test")) // nolint: errcheck
	c.Set("foo", "bar")
	c.reset()

	assert.Equal(t, c.IsAborted(), false)
	assert.Equal(t, c.Keys, nil)
	assert.Equal(t, c.Accepted, nil)
	assert.Equal(t, len(c.Errors), 0)
	assert.Equal(t, len(c.Errors.Errors()), 0)
	assert.Equal(t, len(c.Errors.ByType(ErrorTypeAny)), 0)
	assert.Equal(t, len(c.Params), 0)
	assert.Equal(t, c.index == -1, true)
	assert.Equal(t, c.Writer.(*responseWriter), &c.writermem)
}

func TestContextHandlers(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, c.handlers, nil)
	assert.Equal(t, c.handlers.Last(), nil)

	c.handlers = HandlersChain{}
	assert.NotEqual(t, c.handlers, nil)
	assert.Equal(t, c.handlers.Last(), nil)

	f := func(c *Context) {}
	g := func(c *Context) {}

	c.handlers = HandlersChain{f}
	compareFunc(t, f, c.handlers.Last())

	c.handlers = HandlersChain{f, g}
	compareFunc(t, g, c.handlers.Last())
}

// TestContextSetGet tests that a parameter is set correctly on the
// current context and can be retrieved using Get.
func TestContextSetGet(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("foo", "bar")

	value, err := c.Get("foo")
	assert.Equal(t, "bar", value)
	assert.Equal(t, err, true)

	value, err = c.Get("foo2")
	assert.Equal(t, value, nil)
	assert.Equal(t, err, false)

	assert.Equal(t, "bar", c.MustGet("foo"))
	Panics(t, func() { c.MustGet("no_exist") })
}

func TestContextSetGetValues(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("string", "this is a string")
	c.Set("int32", int32(-42))
	c.Set("int64", int64(42424242424242))
	c.Set("uint64", uint64(42))
	c.Set("float32", float32(4.2))
	c.Set("float64", 4.2)
	var a interface{} = 1
	c.Set("intInterface", a)

	assert.Equal(t, c.MustGet("string").(string), "this is a string")
	assert.Equal(t, c.MustGet("int32").(int32), int32(-42))
	assert.Equal(t, c.MustGet("int64").(int64), int64(42424242424242))
	assert.Equal(t, c.MustGet("uint64").(uint64), uint64(42))
	assert.Equal(t, c.MustGet("float32").(float32), float32(4.2))
	assert.Equal(t, c.MustGet("float64").(float64), 4.2)
	assert.Equal(t, c.MustGet("intInterface").(int), 1)

}

func TestContextGetString(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("string", "this is a string")
	assert.Equal(t, "this is a string", c.GetString("string"))
}

func TestContextSetGetBool(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("bool", true)
	assert.Equal(t, c.GetBool("bool"), true)
}

func TestContextGetInt(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("int", 1)
	assert.Equal(t, 1, c.GetInt("int"))
}

func TestContextGetInt64(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("int64", int64(42424242424242))
	assert.Equal(t, int64(42424242424242), c.GetInt64("int64"))
}

func TestContextGetFloat64(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("float64", 4.2)
	assert.Equal(t, 4.2, c.GetFloat64("float64"))
}

func TestContextGetTime(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	t1, _ := time.Parse("1/2/2006 15:04:05", "01/01/2017 12:00:00")
	c.Set("time", t1)
	assert.Equal(t, t1, c.GetTime("time"))
}

func TestContextGetDuration(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("duration", time.Second)
	assert.Equal(t, time.Second, c.GetDuration("duration"))
}

func TestContextGetStringSlice(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Set("slice", []string{"foo"})
	assert.Equal(t, []string{"foo"}, c.GetStringSlice("slice"))
}

func TestContextGetStringMap(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	var m = make(map[string]interface{})
	m["foo"] = 1
	c.Set("map", m)

	assert.Equal(t, m, c.GetStringMap("map"))
	assert.Equal(t, 1, c.GetStringMap("map")["foo"])
}

func TestContextGetStringMapString(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	var m = make(map[string]string)
	m["foo"] = "bar"
	c.Set("map", m)

	assert.Equal(t, m, c.GetStringMapString("map"))
	assert.Equal(t, "bar", c.GetStringMapString("map")["foo"])
}

func TestContextGetStringMapStringSlice(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	var m = make(map[string][]string)
	m["foo"] = []string{"foo"}
	c.Set("map", m)

	assert.Equal(t, m, c.GetStringMapStringSlice("map"))
	assert.Equal(t, []string{"foo"}, c.GetStringMapStringSlice("map")["foo"])
}

func TestContextCopy(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.index = 2
	c.Request, _ = http.NewRequest("POST", "/hola", nil)
	c.handlers = HandlersChain{func(c *Context) {}}
	c.Params = Params{Param{Key: "foo", Value: "bar"}}
	c.Set("foo", "bar")

	cp := c.Copy()
	assert.Equal(t, cp.handlers, nil)
	assert.Equal(t, cp.writermem.ResponseWriter, nil)
	assert.Equal(t, &cp.writermem, cp.Writer.(*responseWriter))
	assert.Equal(t, cp.Request, c.Request)
	assert.Equal(t, cp.index, abortIndex)
	assert.Equal(t, cp.Keys, c.Keys)
	assert.Equal(t, cp.engine == c.engine, true)
	assert.Equal(t, cp.Params, c.Params)
	cp.Set("foo", "notBar")
	assert.Equal(t, cp.Keys["foo"] == c.Keys["foo"], false)
}

func TestContextHandlerName(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.handlers = HandlersChain{func(c *Context) {}, handlerNameTest}

	assert.MatchRegex(t, "^(.*/vendor/)?github.com/manucorporat/gin-diet.handlerNameTest$", c.HandlerName())
}

func TestContextHandlerNames(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.handlers = HandlersChain{func(c *Context) {}, handlerNameTest, func(c *Context) {}, handlerNameTest2}

	names := c.HandlerNames()

	assert.Equal(t, len(names), 4)
	for _, name := range names {
		assert.MatchRegex(t, name, `^(.*/vendor/)?(github\.com/manucorporat/gin-diet\.){1}(TestContextHandlerNames\.func.*){0,1}(handlerNameTest.*){0,1}`)
	}
}

func handlerNameTest(c *Context) {

}

func handlerNameTest2(c *Context) {

}

var handlerTest HandlerFunc = func(c *Context) {

}

func TestContextHandler(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.handlers = HandlersChain{func(c *Context) {}, handlerTest}

	assert.Equal(t, reflect.ValueOf(handlerTest).Pointer(), reflect.ValueOf(c.Handler()).Pointer())
}

func TestContextQuery(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "http://example.com/?foo=bar&page=10&id=", nil)

	value, ok := c.GetQuery("foo")
	assert.Equal(t, ok, true)
	assert.Equal(t, "bar", value)
	assert.Equal(t, "bar", c.DefaultQuery("foo", "none"))
	assert.Equal(t, "bar", c.Query("foo"))

	value, ok = c.GetQuery("page")
	assert.Equal(t, ok, true)
	assert.Equal(t, "10", value)
	assert.Equal(t, "10", c.DefaultQuery("page", "0"))
	assert.Equal(t, "10", c.Query("page"))

	value, ok = c.GetQuery("id")
	assert.Equal(t, ok, true)
	assert.Equal(t, len(value), 0)
	assert.Equal(t, len(c.DefaultQuery("id", "nada")), 0)
	assert.Equal(t, len(c.Query("id")), 0)

	value, ok = c.GetQuery("NoKey")
	assert.Equal(t, ok, false)
	assert.Equal(t, len(value), 0)
	assert.Equal(t, "nada", c.DefaultQuery("NoKey", "nada"))
	assert.Equal(t, len(c.Query("NoKey")), 0)

	// postform should not mess
	value, ok = c.GetPostForm("page")
	assert.Equal(t, ok, false)
	assert.Equal(t, len(value), 0)
	assert.Equal(t, len(c.PostForm("foo")), 0)
}

func TestContextQueryAndPostForm(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	body := bytes.NewBufferString("foo=bar&page=11&both=&foo=second")
	c.Request, _ = http.NewRequest("POST",
		"/?both=GET&id=main&id=omit&array[]=first&array[]=second&ids[a]=hi&ids[b]=3.14", body)
	c.Request.Header.Add("Content-Type", MIMEPOSTForm)

	assert.Equal(t, "bar", c.DefaultPostForm("foo", "none"))
	assert.Equal(t, "bar", c.PostForm("foo"))
	assert.Equal(t, len(c.Query("foo")), 0)

	value, ok := c.GetPostForm("page")
	assert.Equal(t, ok, true)
	assert.Equal(t, "11", value)
	assert.Equal(t, "11", c.DefaultPostForm("page", "0"))
	assert.Equal(t, "11", c.PostForm("page"))
	assert.Equal(t, len(c.Query("page")), 0)

	value, ok = c.GetPostForm("both")
	assert.Equal(t, ok, true)
	assert.Equal(t, len(value), 0)
	assert.Equal(t, len(c.PostForm("both")), 0)
	assert.Equal(t, len(c.DefaultPostForm("both", "nothing")), 0)
	assert.Equal(t, "GET", c.Query("both"))

	value, ok = c.GetQuery("id")
	assert.Equal(t, ok, true)
	assert.Equal(t, "main", value)
	assert.Equal(t, "000", c.DefaultPostForm("id", "000"))
	assert.Equal(t, "main", c.Query("id"))
	assert.Equal(t, len(c.PostForm("id")), 0)

	value, ok = c.GetQuery("NoKey")
	assert.Equal(t, ok, false)
	assert.Equal(t, len(value), 0)
	value, ok = c.GetPostForm("NoKey")
	assert.Equal(t, ok, false)
	assert.Equal(t, len(value), 0)
	assert.Equal(t, "nada", c.DefaultPostForm("NoKey", "nada"))
	assert.Equal(t, "nothing", c.DefaultQuery("NoKey", "nothing"))
	assert.Equal(t, len(c.PostForm("NoKey")), 0)
	assert.Equal(t, len(c.Query("NoKey")), 0)

	var obj struct {
		Foo   string   `form:"foo"`
		ID    string   `form:"id"`
		Page  int      `form:"page"`
		Both  string   `form:"both"`
		Array []string `form:"array[]"`
	}
	assert.Equal(t, nil, c.Bind(&obj))
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, "main", obj.ID)
	assert.Equal(t, 11, obj.Page)
	assert.Equal(t, 0, len(obj.Both))
	assert.Equal(t, []string{"first", "second"}, obj.Array)

	values, ok := c.GetQueryArray("array[]")
	assert.Equal(t, true, ok)
	assert.Equal(t, "first", values[0])
	assert.Equal(t, "second", values[1])

	values = c.QueryArray("array[]")
	assert.Equal(t, "first", values[0])
	assert.Equal(t, "second", values[1])

	values = c.QueryArray("nokey")
	assert.Equal(t, 0, len(values))

	values = c.QueryArray("both")
	assert.Equal(t, 1, len(values))
	assert.Equal(t, "GET", values[0])

	dicts, ok := c.GetQueryMap("ids")
	assert.Equal(t, true, ok)
	assert.Equal(t, "hi", dicts["a"])
	assert.Equal(t, "3.14", dicts["b"])

	dicts, ok = c.GetQueryMap("nokey")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(dicts))

	dicts, ok = c.GetQueryMap("both")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(dicts))

	dicts, ok = c.GetQueryMap("array")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(dicts))

	dicts = c.QueryMap("ids")
	assert.Equal(t, "hi", dicts["a"])
	assert.Equal(t, "3.14", dicts["b"])

	dicts = c.QueryMap("nokey")
	assert.Equal(t, 0, len(dicts))
}

func TestContextPostFormMultipart(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request = createMultipartRequest()

	var obj struct {
		Foo          string    `form:"foo"`
		Bar          string    `form:"bar"`
		BarAsInt     int       `form:"bar"`
		Array        []string  `form:"array"`
		ID           string    `form:"id"`
		TimeLocal    time.Time `form:"time_local" time_format:"02/01/2006 15:04"`
		TimeUTC      time.Time `form:"time_utc" time_format:"02/01/2006 15:04" time_utc:"1"`
		TimeLocation time.Time `form:"time_location" time_format:"02/01/2006 15:04" time_location:"Asia/Tokyo"`
		BlankTime    time.Time `form:"blank_time" time_format:"02/01/2006 15:04"`
	}
	assert.Equal(t, nil, c.Bind(&obj))
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, "10", obj.Bar)
	assert.Equal(t, 10, obj.BarAsInt)
	assert.Equal(t, []string{"first", "second"}, obj.Array)
	assert.Equal(t, 0, len(obj.ID))
	assert.Equal(t, "31/12/2016 14:55", obj.TimeLocal.Format("02/01/2006 15:04"))
	assert.Equal(t, time.Local, obj.TimeLocal.Location())
	assert.Equal(t, "31/12/2016 14:55", obj.TimeUTC.Format("02/01/2006 15:04"))
	assert.Equal(t, time.UTC, obj.TimeUTC.Location())
	loc, _ := time.LoadLocation("Asia/Tokyo")
	assert.Equal(t, "31/12/2016 14:55", obj.TimeLocation.Format("02/01/2006 15:04"))
	assert.Equal(t, loc, obj.TimeLocation.Location())
	assert.Equal(t, true, obj.BlankTime.IsZero())

	value, ok := c.GetQuery("foo")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(value))
	assert.Equal(t, 0, len(c.Query("bar")))
	assert.Equal(t, "nothing", c.DefaultQuery("id", "nothing"))

	value, ok = c.GetPostForm("foo")
	assert.Equal(t, true, ok)
	assert.Equal(t, "bar", value)
	assert.Equal(t, "bar", c.PostForm("foo"))

	value, ok = c.GetPostForm("array")
	assert.Equal(t, true, ok)
	assert.Equal(t, "first", value)
	assert.Equal(t, "first", c.PostForm("array"))

	assert.Equal(t, "10", c.DefaultPostForm("bar", "nothing"))

	value, ok = c.GetPostForm("id")
	assert.Equal(t, true, ok)
	assert.Equal(t, 0, len(value))
	assert.Equal(t, 0, len(c.PostForm("id")))
	assert.Equal(t, 0, len(c.DefaultPostForm("id", "nothing")))

	value, ok = c.GetPostForm("nokey")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(value))
	assert.Equal(t, "nothing", c.DefaultPostForm("nokey", "nothing"))

	values, ok := c.GetPostFormArray("array")
	assert.Equal(t, true, ok)
	assert.Equal(t, "first", values[0])
	assert.Equal(t, "second", values[1])

	values = c.PostFormArray("array")
	assert.Equal(t, "first", values[0])
	assert.Equal(t, "second", values[1])

	values = c.PostFormArray("nokey")
	assert.Equal(t, 0, len(values))

	values = c.PostFormArray("foo")
	assert.Equal(t, 1, len(values))
	assert.Equal(t, "bar", values[0])

	dicts, ok := c.GetPostFormMap("names")
	assert.Equal(t, true, ok)
	assert.Equal(t, "thinkerou", dicts["a"])
	assert.Equal(t, "tianou", dicts["b"])

	dicts, ok = c.GetPostFormMap("nokey")
	assert.Equal(t, false, ok)
	assert.Equal(t, 0, len(dicts))

	dicts = c.PostFormMap("names")
	assert.Equal(t, "thinkerou", dicts["a"])
	assert.Equal(t, "tianou", dicts["b"])

	dicts = c.PostFormMap("nokey")
	assert.Equal(t, 0, len(dicts))
}

func TestContextSetCookie(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("user", "gin", 1, "/", "localhost", true, true)
	assert.Equal(t, "user=gin; Path=/; Domain=localhost; Max-Age=1; HttpOnly; Secure; SameSite=Lax", c.Writer.Header().Get("Set-Cookie"))
}

func TestContextSetCookiePathEmpty(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("user", "gin", 1, "", "localhost", true, true)
	assert.Equal(t, "user=gin; Path=/; Domain=localhost; Max-Age=1; HttpOnly; Secure; SameSite=Lax", c.Writer.Header().Get("Set-Cookie"))
}

func TestContextGetCookie(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/get", nil)
	c.Request.Header.Set("Cookie", "user=gin")
	cookie, _ := c.Cookie("user")
	assert.Equal(t, "gin", cookie)

	_, err := c.Cookie("nokey")
	assert.NotEqual(t, nil, err)
}

func TestContextBodyAllowedForStatus(t *testing.T) {
	assert.Equal(t, false, bodyAllowedForStatus(http.StatusProcessing))
	assert.Equal(t, false, bodyAllowedForStatus(http.StatusNoContent))
	assert.Equal(t, false, bodyAllowedForStatus(http.StatusNotModified))
	assert.Equal(t, true, bodyAllowedForStatus(http.StatusInternalServerError))
}

type TestPanicRender struct {
}

func (*TestPanicRender) Render(http.ResponseWriter) error {
	return errors.New("TestPanicRender")
}

func (*TestPanicRender) WriteContentType(http.ResponseWriter) {}

func TestContextRenderPanicIfErr(t *testing.T) {
	defer func() {
		r := recover()
		assert.Equal(t, "TestPanicRender", fmt.Sprint(r))
	}()
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Render(http.StatusOK, &TestPanicRender{})

	t.Error("Panic not detected")
}

// Tests that the response is serialized as JSON
// and Content-Type is set to application/json
// and special HTML characters are escaped
func TestContextRenderJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.JSON(http.StatusCreated, H{"foo": "bar", "html": "<b>"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "{\"foo\":\"bar\",\"html\":\"\\u003cb\\u003e\"}", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that the response is serialized as JSONP
// and Content-Type is set to application/javascript
func TestContextRenderJSONP(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "http://example.com/?callback=x", nil)

	c.JSONP(http.StatusCreated, H{"foo": "bar"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "x({\"foo\":\"bar\"});", w.Body.String())
	assert.Equal(t, "application/javascript; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that the response is serialized as JSONP
// and Content-Type is set to application/json
func TestContextRenderJSONPWithoutCallback(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "http://example.com", nil)

	c.JSONP(http.StatusCreated, H{"foo": "bar"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "{\"foo\":\"bar\"}", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no JSON is rendered if code is 204
func TestContextRenderNoContentJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.JSON(http.StatusNoContent, H{"foo": "bar"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that the response is serialized as JSON
// we change the content-type before
func TestContextRenderAPIJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Header("Content-Type", "application/vnd.api+json")
	c.JSON(http.StatusCreated, H{"foo": "bar"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "{\"foo\":\"bar\"}", w.Body.String())
	assert.Equal(t, "application/vnd.api+json", w.Header().Get("Content-Type"))
}

// Tests that no Custom JSON is rendered if code is 204
func TestContextRenderNoContentAPIJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Header("Content-Type", "application/vnd.api+json")
	c.JSON(http.StatusNoContent, H{"foo": "bar"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, w.Header().Get("Content-Type"), "application/vnd.api+json")
}

// Tests that the response is serialized as JSON
// and Content-Type is set to application/json
func TestContextRenderIndentedJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.IndentedJSON(http.StatusCreated, H{"foo": "bar", "bar": "foo", "nested": H{"foo": "bar"}})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "{\n    \"bar\": \"foo\",\n    \"foo\": \"bar\",\n    \"nested\": {\n        \"foo\": \"bar\"\n    }\n}", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no Custom JSON is rendered if code is 204
func TestContextRenderNoContentIndentedJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.IndentedJSON(http.StatusNoContent, H{"foo": "bar", "bar": "foo", "nested": H{"foo": "bar"}})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that the response is serialized as Secure JSON
// and Content-Type is set to application/json
func TestContextRenderSecureJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := CreateTestContext(w)

	router.SecureJsonPrefix("&&&START&&&")
	c.SecureJSON(http.StatusCreated, []string{"foo", "bar"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "&&&START&&&[\"foo\",\"bar\"]", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no Custom JSON is rendered if code is 204
func TestContextRenderNoContentSecureJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.SecureJSON(http.StatusNoContent, []string{"foo", "bar"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextRenderNoContentAsciiJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.AsciiJSON(http.StatusNoContent, []string{"lang", "Go语言"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// Tests that the response is serialized as JSON
// and Content-Type is set to application/json
// and special HTML characters are preserved
func TestContextRenderPureJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.PureJSON(http.StatusCreated, H{"foo": "bar", "html": "<b>"})
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "{\"foo\":\"bar\",\"html\":\"<b>\"}\n", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that the response executes the templates
// and responds with Content-Type set to text/html
func TestContextRenderHTML(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := CreateTestContext(w)

	templ := template.Must(template.New("t").Parse(`Hello {{.name}}`))
	router.SetHTMLTemplate(templ)

	c.HTML(http.StatusCreated, "t", H{"name": "alexandernyquist"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Hello alexandernyquist", w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextRenderHTML2(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := CreateTestContext(w)

	// print debug warning log when Engine.trees > 0
	router.addRoute("GET", "/", HandlersChain{func(_ *Context) {}})
	assert.Equal(t, len(router.trees), 1)

	templ := template.Must(template.New("t").Parse(`Hello {{.name}}`))
	re := captureOutput(t, func() {
		SetMode(DebugMode)
		router.SetHTMLTemplate(templ)
		SetMode(TestMode)
	})

	assert.Equal(t, "[GIN-debug] [WARNING] Since SetHTMLTemplate() is NOT thread-safe. It should only be called\nat initialization. ie. before any route is registered or the router is listening in a socket:\n\n\trouter := gin.Default()\n\trouter.SetHTMLTemplate(template) // << good place\n\n", re)

	c.HTML(http.StatusCreated, "t", H{"name": "alexandernyquist"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Hello alexandernyquist", w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no HTML is rendered if code is 204
func TestContextRenderNoContentHTML(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := CreateTestContext(w)
	templ := template.Must(template.New("t").Parse(`Hello {{.name}}`))
	router.SetHTMLTemplate(templ)

	c.HTML(http.StatusNoContent, "t", H{"name": "alexandernyquist"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestContextXML tests that the response is serialized as XML
// and Content-Type is set to application/xml
func TestContextRenderXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.XML(http.StatusCreated, H{"foo": "bar"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "<map><foo>bar</foo></map>", w.Body.String())
	assert.Equal(t, "application/xml; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no XML is rendered if code is 204
func TestContextRenderNoContentXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.XML(http.StatusNoContent, H{"foo": "bar"})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "application/xml; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestContextString tests that the response is returned
// with Content-Type set to text/plain
func TestContextRenderString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.String(http.StatusCreated, "test %s %d", "string", 2)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "test string 2", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no String is rendered if code is 204
func TestContextRenderNoContentString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.String(http.StatusNoContent, "test %s %d", "string", 2)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestContextString tests that the response is returned
// with Content-Type set to text/html
func TestContextRenderHTMLString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusCreated, "<html>%s %d</html>", "string", 3)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "<html>string 3</html>", w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

// Tests that no HTML String is rendered if code is 204
func TestContextRenderNoContentHTMLString(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusNoContent, "<html>%s %d</html>", "string", 3)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestContextData tests that the response can be written from `bytesting`
// with specified MIME type
func TestContextRenderData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Data(http.StatusCreated, "text/csv", []byte(`foo,bar`))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "foo,bar", w.Body.String())
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
}

// Tests that no Custom Data is rendered if code is 204
func TestContextRenderNoContentData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Data(http.StatusNoContent, "text/csv", []byte(`foo,bar`))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, len(w.Body.String()))
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
}

func TestContextRenderFile(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.File("./gin.go")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, strings.Contains(w.Body.String(), "func New() *Engine {"), true)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextRenderFileFromFS(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("GET", "/some/path", nil)
	c.FileFromFS("./gin.go", Dir(".", false))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, strings.Contains(w.Body.String(), "func New() *Engine {"), true)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "/some/path", c.Request.URL.Path)
}

func TestContextRenderAttachment(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	newFilename := "new_filename.go"

	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.FileAttachment("./gin.go", newFilename)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, strings.Contains(w.Body.String(), "func New() *Engine {"), true)
	assert.Equal(t, fmt.Sprintf("attachment; filename=\"%s\"", newFilename), w.HeaderMap.Get("Content-Disposition"))
}

func TestContextHeaders(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Header("Content-Type", "text/plain")
	c.Header("X-Custom", "value")

	assert.Equal(t, "text/plain", c.Writer.Header().Get("Content-Type"))
	assert.Equal(t, "value", c.Writer.Header().Get("X-Custom"))

	c.Header("Content-Type", "text/html")
	c.Header("X-Custom", "")

	assert.Equal(t, "text/html", c.Writer.Header().Get("Content-Type"))
	_, exist := c.Writer.Header()["X-Custom"]
	assert.Equal(t, false, exist)
}

// TODO
func TestContextRenderRedirectWithRelativePath(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "http://example.com", nil)
	Panics(t, func() { c.Redirect(299, "/new_path") })
	Panics(t, func() { c.Redirect(309, "/new_path") })

	c.Redirect(http.StatusMovedPermanently, "/path")
	c.Writer.WriteHeaderNow()
	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "/path", w.Header().Get("Location"))
}

func TestContextRenderRedirectWithAbsolutePath(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "http://example.com", nil)
	c.Redirect(http.StatusFound, "http://google.com")
	c.Writer.WriteHeaderNow()

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "http://google.com", w.Header().Get("Location"))
}

func TestContextRenderRedirectWith201(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "http://example.com", nil)
	c.Redirect(http.StatusCreated, "/resource")
	c.Writer.WriteHeaderNow()

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "/resource", w.Header().Get("Location"))
}

func TestContextRenderRedirectAll(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "http://example.com", nil)
	Panics(t, func() { c.Redirect(http.StatusOK, "/resource") })
	Panics(t, func() { c.Redirect(http.StatusAccepted, "/resource") })
	Panics(t, func() { c.Redirect(299, "/resource") })
	Panics(t, func() { c.Redirect(309, "/resource") })
	NotPanics(t, func() { c.Redirect(http.StatusMultipleChoices, "/resource") })
	NotPanics(t, func() { c.Redirect(http.StatusPermanentRedirect, "/resource") })
}

func TestContextNegotiationWithJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "", nil)

	c.Negotiate(http.StatusOK, Negotiate{
		Offered: []string{MIMEJSON, MIMEXML},
		Data:    H{"foo": "bar"},
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "{\"foo\":\"bar\"}", w.Body.String())
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextNegotiationWithXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "", nil)

	c.Negotiate(http.StatusOK, Negotiate{
		Offered: []string{MIMEXML, MIMEJSON},
		Data:    H{"foo": "bar"},
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "<map><foo>bar</foo></map>", w.Body.String())
	assert.Equal(t, "application/xml; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextNegotiationWithHTML(t *testing.T) {
	w := httptest.NewRecorder()
	c, router := CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "", nil)
	templ := template.Must(template.New("t").Parse(`Hello {{.name}}`))
	router.SetHTMLTemplate(templ)

	c.Negotiate(http.StatusOK, Negotiate{
		Offered:  []string{MIMEHTML},
		Data:     H{"name": "gin"},
		HTMLName: "t",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello gin", w.Body.String())
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestContextNegotiationNotSupport(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "", nil)

	c.Negotiate(http.StatusOK, Negotiate{
		Offered: []string{MIMEPOSTForm},
	})

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
	assert.Equal(t, c.index == abortIndex, true)

	assert.Equal(t, true, c.IsAborted())
}

func TestContextNegotiationFormat(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "", nil)

	Panics(t, func() { c.NegotiateFormat() })
	assert.Equal(t, MIMEJSON, c.NegotiateFormat(MIMEJSON, MIMEXML))
	assert.Equal(t, MIMEHTML, c.NegotiateFormat(MIMEHTML, MIMEJSON))
}

func TestContextNegotiationFormatWithAccept(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9;q=0.8")

	assert.Equal(t, MIMEXML, c.NegotiateFormat(MIMEJSON, MIMEXML))
	assert.Equal(t, MIMEHTML, c.NegotiateFormat(MIMEXML, MIMEHTML))
	assert.Equal(t, 0, len(c.NegotiateFormat(MIMEJSON)))
}

func TestContextNegotiationFormatWithWildcardAccept(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("Accept", "*/*")

	assert.Equal(t, c.NegotiateFormat("*/*"), "*/*")
	assert.Equal(t, c.NegotiateFormat("text/*"), "text/*")
	assert.Equal(t, c.NegotiateFormat("application/*"), "application/*")
	assert.Equal(t, c.NegotiateFormat(MIMEJSON), MIMEJSON)
	assert.Equal(t, c.NegotiateFormat(MIMEXML), MIMEXML)
	assert.Equal(t, c.NegotiateFormat(MIMEHTML), MIMEHTML)

	c, _ = CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("Accept", "text/*")

	assert.Equal(t, c.NegotiateFormat("*/*"), "*/*")
	assert.Equal(t, c.NegotiateFormat("text/*"), "text/*")
	assert.Equal(t, c.NegotiateFormat("application/*"), "")
	assert.Equal(t, c.NegotiateFormat(MIMEJSON), "")
	assert.Equal(t, c.NegotiateFormat(MIMEXML), "")
	assert.Equal(t, c.NegotiateFormat(MIMEHTML), MIMEHTML)
}

func TestContextNegotiationFormatCustom(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9;q=0.8")

	c.Accepted = nil
	c.SetAccepted(MIMEJSON, MIMEXML)

	assert.Equal(t, MIMEJSON, c.NegotiateFormat(MIMEJSON, MIMEXML))
	assert.Equal(t, MIMEXML, c.NegotiateFormat(MIMEXML, MIMEHTML))
	assert.Equal(t, MIMEJSON, c.NegotiateFormat(MIMEJSON))
}

func TestContextIsAborted(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, false, c.IsAborted())

	c.Abort()
	assert.Equal(t, true, c.IsAborted())

	c.Next()
	assert.Equal(t, true, c.IsAborted())

	c.index++
	assert.Equal(t, true, c.IsAborted())
}

// TestContextData tests that the response can be written from `bytesting`
// with specified MIME type
func TestContextAbortWithStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.index = 4
	c.AbortWithStatus(http.StatusUnauthorized)

	assert.Equal(t, abortIndex == c.index, true)
	assert.Equal(t, http.StatusUnauthorized, c.Writer.Status())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, true, c.IsAborted())
}

type testJSONAbortMsg struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

func TestContextAbortWithStatusJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)
	c.index = 4

	in := new(testJSONAbortMsg)
	in.Bar = "barValue"
	in.Foo = "fooValue"

	c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, in)

	assert.Equal(t, abortIndex == c.index, true)
	assert.Equal(t, http.StatusUnsupportedMediaType, c.Writer.Status())
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	assert.Equal(t, true, c.IsAborted())

	contentType := w.Header().Get("Content-Type")
	assert.Equal(t, "application/json; charset=utf-8", contentType)

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(w.Body)
	assert.Equal(t, nil, err)
	jsonStringBody := buf.String()
	assert.Equal(t, fmt.Sprint("{\"foo\":\"fooValue\",\"bar\":\"barValue\"}"), jsonStringBody)
}

func TestContextError(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, 0, len(c.Errors))

	firstErr := errors.New("first error")
	c.Error(firstErr) // nolint: errcheck
	assert.Equal(t, len(c.Errors), 1)
	assert.Equal(t, "Error #01: first error\n", c.Errors.String())

	secondErr := errors.New("second error")
	c.Error(&Error{ // nolint: errcheck
		Err:  secondErr,
		Meta: "some data 2",
		Type: ErrorTypePublic,
	})
	assert.Equal(t, len(c.Errors), 2)

	assert.Equal(t, firstErr, c.Errors[0].Err)
	assert.Equal(t, nil, c.Errors[0].Meta)
	assert.Equal(t, ErrorTypePrivate, c.Errors[0].Type)

	assert.Equal(t, secondErr, c.Errors[1].Err)
	assert.Equal(t, "some data 2", c.Errors[1].Meta)
	assert.Equal(t, ErrorTypePublic, c.Errors[1].Type)

	assert.Equal(t, c.Errors.Last(), c.Errors[1])

	defer func() {
		if recover() == nil {
			t.Error("didn't panic")
		}
	}()
	c.Error(nil) // nolint: errcheck
}

func TestContextTypedError(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Error(errors.New("externo 0")).SetType(ErrorTypePublic)  // nolint: errcheck
	c.Error(errors.New("interno 0")).SetType(ErrorTypePrivate) // nolint: errcheck

	for _, err := range c.Errors.ByType(ErrorTypePublic) {
		assert.Equal(t, ErrorTypePublic, err.Type)
	}
	for _, err := range c.Errors.ByType(ErrorTypePrivate) {
		assert.Equal(t, ErrorTypePrivate, err.Type)
	}
	assert.Equal(t, []string{"externo 0", "interno 0"}, c.Errors.Errors())
}

func TestContextAbortWithError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.AbortWithError(http.StatusUnauthorized, errors.New("bad input")).SetMeta("some input") // nolint: errcheck

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, abortIndex == c.index, true)
	assert.Equal(t, true, c.IsAborted())
}

func TestContextClientIP(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)

	c.Request.Header.Set("X-Real-IP", " 10.10.10.10  ")
	c.Request.Header.Set("X-Forwarded-For", "  20.20.20.20, 30.30.30.30")
	c.Request.Header.Set("X-Appengine-Remote-Addr", "50.50.50.50")
	c.Request.RemoteAddr = "  40.40.40.40:42123 "

	assert.Equal(t, "20.20.20.20", c.ClientIP())

	c.Request.Header.Del("X-Forwarded-For")
	assert.Equal(t, "10.10.10.10", c.ClientIP())

	c.Request.Header.Set("X-Forwarded-For", "30.30.30.30  ")
	assert.Equal(t, "30.30.30.30", c.ClientIP())

	c.Request.Header.Del("X-Forwarded-For")
	c.Request.Header.Del("X-Real-IP")
	c.engine.AppEngine = true
	assert.Equal(t, "50.50.50.50", c.ClientIP())

	c.Request.Header.Del("X-Appengine-Remote-Addr")
	assert.Equal(t, "40.40.40.40", c.ClientIP())

	// no port
	c.Request.RemoteAddr = "50.50.50.50"
	assert.Equal(t, 0, len(c.ClientIP()))
}

func TestContextContentType(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Set("Content-Type", "application/json; charset=utf-8")

	assert.Equal(t, "application/json", c.ContentType())
}

func TestContextAutoBindJSON(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString("{\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEJSON)

	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	assert.Equal(t, nil, c.Bind(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, 0, len(c.Errors))
}

func TestContextBindWithJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString("{\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEXML) // set fake content-type

	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	assert.Equal(t, nil, c.BindJSON(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, 0, w.Body.Len())
}
func TestContextBindWithXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString(`<?xml version="1.0" encoding="UTF-8"?>
		<root>
			<foo>FOO</foo>
		   	<bar>BAR</bar>
		</root>`))
	c.Request.Header.Add("Content-Type", MIMEXML) // set fake content-type

	var obj struct {
		Foo string `xml:"foo"`
		Bar string `xml:"bar"`
	}
	assert.Equal(t, nil, c.BindXML(&obj))
	assert.Equal(t, "FOO", obj.Foo)
	assert.Equal(t, "BAR", obj.Bar)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextBindHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("rate", "8000")
	c.Request.Header.Add("domain", "music")
	c.Request.Header.Add("limit", "1000")

	var testHeader struct {
		Rate   int    `header:"Rate"`
		Domain string `header:"Domain"`
		Limit  int    `header:"limit"`
	}

	assert.Equal(t, nil, c.BindHeader(&testHeader))
	assert.Equal(t, 8000, testHeader.Rate)
	assert.Equal(t, "music", testHeader.Domain)
	assert.Equal(t, 1000, testHeader.Limit)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextBindWithQuery(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/?foo=bar&bar=foo", bytes.NewBufferString("foo=unused"))

	var obj struct {
		Foo string `form:"foo"`
		Bar string `form:"bar"`
	}
	assert.Equal(t, nil, c.BindQuery(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextBadAutoBind(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "http://example.com", bytes.NewBufferString("\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEJSON)
	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	assert.Equal(t, false, c.IsAborted())
	assert.NotEqual(t, nil, c.Bind(&obj))
	c.Writer.WriteHeaderNow()

	assert.Equal(t, 0, len(obj.Bar))
	assert.Equal(t, 0, len(obj.Foo))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, true, c.IsAborted())
}

func TestContextAutoShouldBindJSON(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString("{\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEJSON)

	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	assert.Equal(t, nil, c.ShouldBind(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, 0, len(c.Errors))
}

func TestContextShouldBindWithJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString("{\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEXML) // set fake content-type

	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	assert.Equal(t, nil, c.ShouldBindJSON(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextShouldBindWithXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString(`<?xml version="1.0" encoding="UTF-8"?>
		<root>
			<foo>FOO</foo>
			<bar>BAR</bar>
		</root>`))
	c.Request.Header.Add("Content-Type", MIMEXML) // set fake content-type

	var obj struct {
		Foo string `xml:"foo"`
		Bar string `xml:"bar"`
	}
	assert.Equal(t, c.ShouldBindXML(&obj), nil)
	assert.Equal(t, "FOO", obj.Foo)
	assert.Equal(t, "BAR", obj.Bar)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextShouldBindHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.Header.Add("rate", "8000")
	c.Request.Header.Add("domain", "music")
	c.Request.Header.Add("limit", "1000")

	var testHeader struct {
		Rate   int    `header:"Rate"`
		Domain string `header:"Domain"`
		Limit  int    `header:"limit"`
	}

	assert.Equal(t, nil, c.ShouldBindHeader(&testHeader))
	assert.Equal(t, 8000, testHeader.Rate)
	assert.Equal(t, "music", testHeader.Domain)
	assert.Equal(t, 1000, testHeader.Limit)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextShouldBindWithQuery(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/?foo=bar&bar=foo&Foo=bar1&Bar=foo1", bytes.NewBufferString("foo=unused"))

	var obj struct {
		Foo  string `form:"foo"`
		Bar  string `form:"bar"`
		Foo1 string `form:"Foo"`
		Bar1 string `form:"Bar"`
	}
	assert.Equal(t, nil, c.ShouldBindQuery(&obj))
	assert.Equal(t, "foo", obj.Bar)
	assert.Equal(t, "bar", obj.Foo)
	assert.Equal(t, "foo1", obj.Bar1)
	assert.Equal(t, "bar1", obj.Foo1)
	assert.Equal(t, 0, w.Body.Len())
}

func TestContextBadAutoShouldBind(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "http://example.com", bytes.NewBufferString("\"foo\":\"bar\", \"bar\":\"foo\"}"))
	c.Request.Header.Add("Content-Type", MIMEJSON)
	var obj struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}

	assert.Equal(t, false, c.IsAborted())
	assert.NotEqual(t, nil, c.ShouldBind(&obj))

	assert.Equal(t, 0, len(obj.Bar))
	assert.Equal(t, 0, len(obj.Foo))
	assert.Equal(t, false, c.IsAborted())
}

func TestContextShouldBindBodyWith(t *testing.T) {
	type typeA struct {
		Foo string `json:"foo" xml:"foo" binding:"required"`
	}
	type typeB struct {
		Bar string `json:"bar" xml:"bar" binding:"required"`
	}
	for _, tt := range []struct {
		name               string
		bindingA, bindingB binding.BindingBody
		bodyA, bodyB       string
	}{
		{
			name:     "JSON & JSON",
			bindingA: binding.JSON,
			bindingB: binding.JSON,
			bodyA:    `{"foo":"FOO"}`,
			bodyB:    `{"bar":"BAR"}`,
		},
		{
			name:     "JSON & XML",
			bindingA: binding.JSON,
			bindingB: binding.XML,
			bodyA:    `{"foo":"FOO"}`,
			bodyB: `<?xml version="1.0" encoding="UTF-8"?>
<root>
   <bar>BAR</bar>
</root>`,
		},
		{
			name:     "XML & XML",
			bindingA: binding.XML,
			bindingB: binding.XML,
			bodyA: `<?xml version="1.0" encoding="UTF-8"?>
<root>
   <foo>FOO</foo>
</root>`,
			bodyB: `<?xml version="1.0" encoding="UTF-8"?>
<root>
   <bar>BAR</bar>
</root>`,
		},
	} {
		t.Logf("testing: %s", tt.name)
		// bodyA to typeA and typeB
		{
			w := httptest.NewRecorder()
			c, _ := CreateTestContext(w)
			c.Request, _ = http.NewRequest(
				"POST", "http://example.com", bytes.NewBufferString(tt.bodyA),
			)
			// When it binds to typeA and typeB, it finds the body is
			// not typeB but typeA.
			objA := typeA{}
			assert.Equal(t, nil, c.ShouldBindBodyWith(&objA, tt.bindingA))
			assert.Equal(t, typeA{"FOO"}, objA)
		}
		// bodyB to typeA and typeB
		{
			// When it binds to typeA and typeB, it finds the body is
			// not typeA but typeB.
			w := httptest.NewRecorder()
			c, _ := CreateTestContext(w)
			c.Request, _ = http.NewRequest(
				"POST", "http://example.com", bytes.NewBufferString(tt.bodyB),
			)
			objB := typeB{}
			assert.Equal(t, nil, c.ShouldBindBodyWith(&objB, tt.bindingB))
			assert.Equal(t, typeB{"BAR"}, objB)
		}
	}
}

func TestContextGolangContext(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewBufferString("{\"foo\":\"bar\", \"bar\":\"foo\"}"))
	assert.Equal(t, nil, c.Err())
	assert.Equal(t, nil, c.Done())
	ti, ok := c.Deadline()
	assert.Equal(t, ti, time.Time{})
	assert.Equal(t, false, ok)
	assert.Equal(t, c.Value(0) == c.Request, true)
	assert.Equal(t, nil, c.Value("foo"))

	c.Set("foo", "bar")
	assert.Equal(t, "bar", c.Value("foo"))
	assert.Equal(t, nil, c.Value(1))
}

func TestWebsocketsRequired(t *testing.T) {
	// Example request from spec: https://tools.ietf.org/html/rfc6455#section-1.2
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/chat", nil)
	c.Request.Header.Set("Host", "server.example.com")
	c.Request.Header.Set("Upgrade", "websocket")
	c.Request.Header.Set("Connection", "Upgrade")
	c.Request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	c.Request.Header.Set("Origin", "http://example.com")
	c.Request.Header.Set("Sec-WebSocket-Protocol", "chat, superchat")
	c.Request.Header.Set("Sec-WebSocket-Version", "13")

	assert.Equal(t, true, c.IsWebsocket())

	// Normal request, no websocket required.
	c, _ = CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/chat", nil)
	c.Request.Header.Set("Host", "server.example.com")

	assert.Equal(t, false, c.IsWebsocket())
}

func TestGetRequestHeaderValue(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/chat", nil)
	c.Request.Header.Set("Gin-Version", "1.0.0")

	assert.Equal(t, "1.0.0", c.GetHeader("Gin-Version"))
	assert.Equal(t, 0, len(c.GetHeader("Connection")))
}

func TestContextGetRawData(t *testing.T) {
	c, _ := CreateTestContext(httptest.NewRecorder())
	body := bytes.NewBufferString("Fetch binary post data")
	c.Request, _ = http.NewRequest("POST", "/", body)
	c.Request.Header.Add("Content-Type", MIMEPOSTForm)

	data, err := c.GetRawData()
	assert.Equal(t, nil, err)
	assert.Equal(t, "Fetch binary post data", string(data))
}

func TestContextRenderDataFromReader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	body := "#!PNG some raw data"
	reader := strings.NewReader(body)
	contentLength := int64(len(body))
	contentType := "image/png"
	extraHeaders := map[string]string{"Content-Disposition": `attachment; filename="gopher.png"`}

	c.DataFromReader(http.StatusOK, contentLength, contentType, reader, extraHeaders)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, w.Body.String())
	assert.Equal(t, contentType, w.Header().Get("Content-Type"))
	assert.Equal(t, fmt.Sprintf("%d", contentLength), w.Header().Get("Content-Length"))
	assert.Equal(t, extraHeaders["Content-Disposition"], w.Header().Get("Content-Disposition"))
}

func TestContextRenderDataFromReaderNoHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := CreateTestContext(w)

	body := "#!PNG some raw data"
	reader := strings.NewReader(body)
	contentLength := int64(len(body))
	contentType := "image/png"

	c.DataFromReader(http.StatusOK, contentLength, contentType, reader, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, body, w.Body.String())
	assert.Equal(t, contentType, w.Header().Get("Content-Type"))
	assert.Equal(t, fmt.Sprintf("%d", contentLength), w.Header().Get("Content-Length"))
}

type TestResponseRecorder struct {
	*httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *TestResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func (r *TestResponseRecorder) closeClient() {
	r.closeChannel <- true
}

func CreateTestResponseRecorder() *TestResponseRecorder {
	return &TestResponseRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func TestContextStream(t *testing.T) {
	w := CreateTestResponseRecorder()
	c, _ := CreateTestContext(w)

	stopStream := true
	c.Stream(func(w io.Writer) bool {
		defer func() {
			stopStream = false
		}()

		_, err := w.Write([]byte("test"))
		assert.Equal(t, nil, err)

		return stopStream
	})

	assert.Equal(t, "testtest", w.Body.String())
}

func TestContextStreamWithClientGone(t *testing.T) {
	w := CreateTestResponseRecorder()
	c, _ := CreateTestContext(w)

	c.Stream(func(writer io.Writer) bool {
		defer func() {
			w.closeClient()
		}()

		_, err := writer.Write([]byte("test"))
		assert.Equal(t, nil, err)

		return true
	})

	assert.Equal(t, "test", w.Body.String())
}

func TestContextResetInHandler(t *testing.T) {
	w := CreateTestResponseRecorder()
	c, _ := CreateTestContext(w)

	c.handlers = []HandlerFunc{
		func(c *Context) { c.reset() },
	}
	NotPanics(t, func() {
		c.Next()
	})
}

func TestRaceParamsContextCopy(t *testing.T) {
	DefaultWriter = os.Stdout
	router := Default()
	nameGroup := router.Group("/:name")
	var wg sync.WaitGroup
	wg.Add(2)
	{
		nameGroup.GET("/api", func(c *Context) {
			go func(c *Context, param string) {
				defer wg.Done()
				// First assert must be executed after the second request
				time.Sleep(50 * time.Millisecond)
				assert.Equal(t, c.Param("name"), param)
			}(c.Copy(), c.Param("name"))
		})
	}
	performRequest(router, "GET", "/name1/api")
	performRequest(router, "GET", "/name2/api")
	wg.Wait()
}

func TestContextWithKeysMutex(t *testing.T) {
	c := &Context{}
	c.Set("foo", "bar")

	value, err := c.Get("foo")
	assert.Equal(t, "bar", value)
	assert.Equal(t, true, err)

	value, err = c.Get("foo2")
	assert.Equal(t, nil, value)
	assert.Equal(t, false, err)
}
