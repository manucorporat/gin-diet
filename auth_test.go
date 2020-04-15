// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/assert"
)

func TestBasicAuth(t *testing.T) {
	pairs := processAccounts(Accounts{
		"admin": "password",
		"foo":   "bar",
		"bar":   "foo",
	})

	assert.Equal(t, len(pairs), 3)
	assert.Equal(t, pairs[2], authPair{
		user:  "bar",
		value: "Basic YmFyOmZvbw==",
	})
	assert.Equal(t, pairs[1], authPair{
		user:  "foo",
		value: "Basic Zm9vOmJhcg==",
	})
	assert.Equal(t, pairs[0], authPair{
		user:  "admin",
		value: "Basic YWRtaW46cGFzc3dvcmQ=",
	})
}

func TestBasicAuthFails(t *testing.T) {
	Panics(t, func() { processAccounts(nil) })
	Panics(t, func() {
		processAccounts(Accounts{
			"":    "password",
			"foo": "bar",
		})
	})
}

func TestBasicAuthSearchCredential(t *testing.T) {
	pairs := processAccounts(Accounts{
		"admin": "password",
		"foo":   "bar",
		"bar":   "foo",
	})

	user, found := pairs.searchCredential(authorizationHeader("admin", "password"))
	assert.Equal(t, "admin", user)
	assert.Equal(t, found, true)

	user, found = pairs.searchCredential(authorizationHeader("foo", "bar"))
	assert.Equal(t, "foo", user)
	assert.Equal(t, found, true)

	user, found = pairs.searchCredential(authorizationHeader("bar", "foo"))
	assert.Equal(t, "bar", user)
	assert.Equal(t, found, true)

	user, found = pairs.searchCredential(authorizationHeader("admins", "password"))
	assert.Equal(t, len(user), 0)
	assert.Equal(t, found, false)

	user, found = pairs.searchCredential(authorizationHeader("foo", "bar "))
	assert.Equal(t, len(user), 0)
	assert.Equal(t, found, false)

	user, found = pairs.searchCredential("")
	assert.Equal(t, len(user), 0)
	assert.Equal(t, found, false)
}

func TestBasicAuthAuthorizationHeader(t *testing.T) {
	assert.Equal(t, "Basic YWRtaW46cGFzc3dvcmQ=", authorizationHeader("admin", "password"))
}

func TestBasicAuthSucceed(t *testing.T) {
	accounts := Accounts{"admin": "password"}
	router := New()
	router.Use(BasicAuth(accounts))
	router.GET("/login", func(c *Context) {
		c.String(http.StatusOK, c.MustGet(AuthUserKey).(string))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	req.Header.Set("Authorization", authorizationHeader("admin", "password"))
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin", w.Body.String())
}

func TestBasicAuth401(t *testing.T) {
	called := false
	accounts := Accounts{"foo": "bar"}
	router := New()
	router.Use(BasicAuth(accounts))
	router.GET("/login", func(c *Context) {
		called = true
		c.String(http.StatusOK, c.MustGet(AuthUserKey).(string))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:password")))
	router.ServeHTTP(w, req)

	assert.Equal(t, called, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "Basic realm=\"Authorization Required\"", w.Header().Get("WWW-Authenticate"))
}

func TestBasicAuth401WithCustomRealm(t *testing.T) {
	called := false
	accounts := Accounts{"foo": "bar"}
	router := New()
	router.Use(BasicAuthForRealm(accounts, "My Custom \"Realm\""))
	router.GET("/login", func(c *Context) {
		called = true
		c.String(http.StatusOK, c.MustGet(AuthUserKey).(string))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:password")))
	router.ServeHTTP(w, req)

	assert.Equal(t, called, false)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "Basic realm=\"My Custom \\\"Realm\\\"\"", w.Header().Get("WWW-Authenticate"))
}
