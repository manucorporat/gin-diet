// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"net/http"
	"testing"

	"github.com/go-playground/assert"
)

func init() {
	SetMode(TestMode)
}

func TestRouterGroupBasic(t *testing.T) {
	router := New()
	group := router.Group("/hola", func(c *Context) {})
	group.Use(func(c *Context) {})

	assert.Equal(t, len(group.Handlers), 2)
	assert.Equal(t, "/hola", group.BasePath())
	assert.Equal(t, router == group.engine, true)

	group2 := group.Group("manu")
	group2.Use(func(c *Context) {}, func(c *Context) {})

	assert.Equal(t, len(group2.Handlers), 4)
	assert.Equal(t, "/hola/manu", group2.BasePath())
	assert.Equal(t, router == group2.engine, true)
}

func TestRouterGroupBasicHandle(t *testing.T) {
	performRequestInGroup(t, http.MethodGet)
	performRequestInGroup(t, http.MethodPost)
	performRequestInGroup(t, http.MethodPut)
	performRequestInGroup(t, http.MethodPatch)
	performRequestInGroup(t, http.MethodDelete)
	performRequestInGroup(t, http.MethodHead)
	performRequestInGroup(t, http.MethodOptions)
}

func performRequestInGroup(t *testing.T, method string) {
	router := New()
	v1 := router.Group("v1", func(c *Context) {})
	assert.Equal(t, "/v1", v1.BasePath())

	login := v1.Group("/login/", func(c *Context) {}, func(c *Context) {})
	assert.Equal(t, "/v1/login/", login.BasePath())

	handler := func(c *Context) {
		c.String(http.StatusBadRequest, "the method was %s and index %d", c.Request.Method, c.index)
	}

	switch method {
	case http.MethodGet:
		v1.GET("/test", handler)
		login.GET("/test", handler)
	case http.MethodPost:
		v1.POST("/test", handler)
		login.POST("/test", handler)
	case http.MethodPut:
		v1.PUT("/test", handler)
		login.PUT("/test", handler)
	case http.MethodPatch:
		v1.PATCH("/test", handler)
		login.PATCH("/test", handler)
	case http.MethodDelete:
		v1.DELETE("/test", handler)
		login.DELETE("/test", handler)
	case http.MethodHead:
		v1.HEAD("/test", handler)
		login.HEAD("/test", handler)
	case http.MethodOptions:
		v1.OPTIONS("/test", handler)
		login.OPTIONS("/test", handler)
	default:
		panic("unknown method")
	}

	w := performRequest(router, method, "/v1/login/test")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "the method was "+method+" and index 3", w.Body.String())

	w = performRequest(router, method, "/v1/test")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "the method was "+method+" and index 1", w.Body.String())
}

func TestRouterGroupInvalidStatic(t *testing.T) {
	router := New()
	Panics(t, func() {
		router.Static("/path/:param", "/")
	})

	Panics(t, func() {
		router.Static("/path/*param", "/")
	})
}

func TestRouterGroupInvalidStaticFile(t *testing.T) {
	router := New()
	Panics(t, func() {
		router.StaticFile("/path/:param", "favicon.ico")
	})

	Panics(t, func() {
		router.StaticFile("/path/*param", "favicon.ico")
	})
}

func TestRouterGroupTooManyHandlers(t *testing.T) {
	router := New()
	handlers1 := make([]HandlerFunc, 40)
	router.Use(handlers1...)

	handlers2 := make([]HandlerFunc, 26)
	Panics(t, func() {
		router.Use(handlers2...)
	})
	Panics(t, func() {
		router.GET("/", handlers2...)
	})
}

func TestRouterGroupBadMethod(t *testing.T) {
	router := New()
	Panics(t, func() {
		router.Handle(http.MethodGet, "/")
	})
	Panics(t, func() {
		router.Handle(" GET", "/")
	})
	Panics(t, func() {
		router.Handle("GET ", "/")
	})
	Panics(t, func() {
		router.Handle("", "/")
	})
	Panics(t, func() {
		router.Handle("PO ST", "/")
	})
	Panics(t, func() {
		router.Handle("1GET", "/")
	})
	Panics(t, func() {
		router.Handle("PATCh", "/")
	})
}

func TestRouterGroupPipeline(t *testing.T) {
	router := New()
	testRoutesInterface(t, router)

	v1 := router.Group("/v1")
	testRoutesInterface(t, v1)
}

func testRoutesInterface(t *testing.T, r IRoutes) {
	handler := func(c *Context) {}
	assert.Equal(t, r == r.Use(handler), true)

	assert.Equal(t, r == r.Handle(http.MethodGet, "/handler", handler), true)
	assert.Equal(t, true, r == r.Any("/any", handler))
	assert.Equal(t, true, r == r.GET("/", handler))
	assert.Equal(t, true, r == r.POST("/", handler))
	assert.Equal(t, true, r == r.DELETE("/", handler))
	assert.Equal(t, true, r == r.PATCH("/", handler))
	assert.Equal(t, true, r == r.PUT("/", handler))
	assert.Equal(t, true, r == r.OPTIONS("/", handler))
	assert.Equal(t, true, r == r.HEAD("/", handler))

	assert.Equal(t, true, r == r.StaticFile("/file", "."))
	assert.Equal(t, true, r == r.Static("/static", "."))
	assert.Equal(t, true, r == r.StaticFS("/static2", Dir(".", false)))
}
