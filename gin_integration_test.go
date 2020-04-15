// Copyright 2017 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package gin

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/assert"
)

func testRequest(t *testing.T, url string) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(url)
	assert.Equal(t, nil, err)
	defer resp.Body.Close()

	body, ioerr := ioutil.ReadAll(resp.Body)
	assert.Equal(t, nil, ioerr)
	assert.Equal(t, "it worked", string(body))
	assert.Equal(t, "200 OK", resp.Status)
}

func TestRunEmpty(t *testing.T) {
	os.Setenv("PORT", "")
	router := New()
	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.Run())
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	assert.NotEqual(t, nil, router.Run(":8080"))
	testRequest(t, "http://localhost:8080/example")
}

func TestRunTLS(t *testing.T) {
	router := New()
	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })

		assert.Equal(t, nil, router.RunTLS(":8443", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"))
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	assert.NotEqual(t, nil, router.RunTLS(":8443", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"))
	testRequest(t, "https://localhost:8443/example")
}

func TestPusher(t *testing.T) {
	var html = template.Must(template.New("https").Parse(`
<html>
<head>
  <title>Https Test</title>
  <script src="/assets/app.js"></script>
</head>
<body>
  <h1 style="color:red;">Welcome, Ginner!</h1>
</body>
</html>
`))

	router := New()
	router.Static("./assets", "./assets")
	router.SetHTMLTemplate(html)

	go func() {
		router.GET("/pusher", func(c *Context) {
			if pusher := c.Writer.Pusher(); pusher != nil {
				err := pusher.Push("/assets/app.js", nil)
				assert.Equal(t, nil, err)
			}
			c.String(http.StatusOK, "it worked")
		})

		assert.Equal(t, nil, router.RunTLS(":8449", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"))
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	assert.NotEqual(t, nil, router.RunTLS(":8449", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"))
	testRequest(t, "https://localhost:8449/pusher")
}

func TestRunEmptyWithEnv(t *testing.T) {
	os.Setenv("PORT", "3123")
	router := New()
	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.Run())
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	assert.NotEqual(t, nil, router.Run(":3123"))
	testRequest(t, "http://localhost:3123/example")
}

func TestRunTooMuchParams(t *testing.T) {
	router := New()
	Panics(t, func() {
		assert.Equal(t, nil, router.Run("2", "2"))
	})
}

func TestRunWithPort(t *testing.T) {
	router := New()
	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.Run(":5150"))
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	assert.NotEqual(t, nil, router.Run(":5150"))
	testRequest(t, "http://localhost:5150/example")
}

func TestUnixSocket(t *testing.T) {
	router := New()

	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.RunUnix("/tmp/unix_unit_test"))
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	c, err := net.Dial("unix", "/tmp/unix_unit_test")
	assert.Equal(t, nil, err)

	fmt.Fprint(c, "GET /example HTTP/1.0\r\n\r\n")
	scanner := bufio.NewScanner(c)
	var response string
	for scanner.Scan() {
		response += scanner.Text()
	}
	assert.Equal(t, strings.Contains(response, "HTTP/1.0 200"), true)
	assert.Equal(t, strings.Contains(response, "it worked"), true)
}

func TestBadUnixSocket(t *testing.T) {
	router := New()
	assert.NotEqual(t, nil, router.RunUnix("#/tmp/unix_unit_test"))
}

func TestFileDescriptor(t *testing.T) {
	router := New()

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.Equal(t, nil, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.Equal(t, nil, err)
	socketFile, err := listener.File()
	assert.Equal(t, nil, err)

	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.RunFd(int(socketFile.Fd())))
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	c, err := net.Dial("tcp", listener.Addr().String())
	assert.Equal(t, nil, err)

	fmt.Fprintf(c, "GET /example HTTP/1.0\r\n\r\n")
	scanner := bufio.NewScanner(c)
	var response string
	for scanner.Scan() {
		response += scanner.Text()
	}
	Contains(t, response, "HTTP/1.0 200")
	Contains(t, response, "it worked")
}

func TestBadFileDescriptor(t *testing.T) {
	router := New()
	assert.NotEqual(t, nil, router.RunFd(0))
}

func TestListener(t *testing.T) {
	router := New()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.Equal(t, nil, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.Equal(t, nil, err)
	go func() {
		router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })
		assert.Equal(t, nil, router.RunListener(listener))
	}()
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	c, err := net.Dial("tcp", listener.Addr().String())
	assert.Equal(t, nil, err)

	fmt.Fprintf(c, "GET /example HTTP/1.0\r\n\r\n")
	scanner := bufio.NewScanner(c)
	var response string
	for scanner.Scan() {
		response += scanner.Text()
	}
	Contains(t, response, "HTTP/1.0 200")
	Contains(t, response, "it worked")
}

func TestBadListener(t *testing.T) {
	router := New()
	addr, err := net.ResolveTCPAddr("tcp", "localhost:10086")
	assert.Equal(t, nil, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.Equal(t, nil, err)
	listener.Close()
	assert.NotEqual(t, nil, router.RunListener(listener))
}

func TestWithHttptestWithAutoSelectedPort(t *testing.T) {
	router := New()
	router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })

	ts := httptest.NewServer(router)
	defer ts.Close()

	testRequest(t, ts.URL+"/example")
}

func TestConcurrentHandleContext(t *testing.T) {
	router := New()
	router.GET("/", func(c *Context) {
		c.Request.URL.Path = "/example"
		router.HandleContext(c)
	})
	router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })

	var wg sync.WaitGroup
	iterations := 200
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			testGetRequestHandler(t, router, "/")
			wg.Done()
		}()
	}
	wg.Wait()
}

// func TestWithHttptestWithSpecifiedPort(t *testing.T) {
// 	router := New()
// 	router.GET("/example", func(c *Context) { c.String(http.StatusOK, "it worked") })

// 	l, _ := net.Listen("tcp", ":8033")
// 	ts := httptest.Server{
// 		Listener: l,
// 		Config:   &http.Server{Handler: router},
// 	}
// 	ts.Start()
// 	defer ts.Close()

// 	testRequest(t, "http://localhost:8033/example")
// }

func testGetRequestHandler(t *testing.T, h http.Handler, url string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	assert.Equal(t, nil, err)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, "it worked", w.Body.String())
	assert.Equal(t, 200, w.Code)
}
