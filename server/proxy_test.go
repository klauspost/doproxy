package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/doproxy/server/httpmock"
)

type mockBackend struct {
	*backend
	n int
}

// ID returns a unique ID of this backend
func (d *mockBackend) ID() string {
	return fmt.Sprintf("id%d", d.n)
}

// ID returns a name of this backend
func (d *mockBackend) Name() string {
	return fmt.Sprintf("mockBackend%d", d.n)
}

var defaultConfig *Config
var loadOnce sync.Once

func newMockBackend(t *testing.T, n int) Backend {
	loadOnce.Do(func() {
		var err error
		defaultConfig, err = ReadConfigFile("testdata/validconfig.toml")
		if err != nil {
			t.Fatal("Unable to read config:", err)
		}
	})
	b := &mockBackend{
		backend: newBackend(defaultConfig.Backend, "", ""),
		n:       n,
	}
	b.rt.mu.Lock()
	defer b.rt.mu.Unlock()
	b.rt.rt = httpmock.DefaultMockTransport
	return b
}

func newMockInventory(t *testing.T, n int) *Inventory {
	if n <= 0 {
		return NewInventory([]Backend{}, BackendConfig{})
	}
	var be = make([]Backend, n)
	for i := 0; i < n; i++ {
		be[i] = newMockBackend(t, i)
	}
	return NewInventory(be, defaultConfig.Backend)
}

// Test a simple roundtrip
func TestProxyRoundtrip(t *testing.T) {
	inv := newMockInventory(t, 3)
	var respOK = make(chan bool, 1)
	responder := func(req *http.Request) (*http.Response, error) {
		t.Log("Path:", req.URL.Path)
		respOK <- req.URL.Path == "/somepath"
		return httpmock.MockResponse(req)
	}
	httpmock.RegisterResponder("GET", responder)

	lb, err := NewLoadBalancer(defaultConfig.LoadBalancing, inv)
	if err != nil {
		t.Fatal(err)
	}
	proxy := NewReverseProxyConfig(*defaultConfig, lb)

	ts := httptest.NewServer(proxy)
	defer ts.Close()
	res, err := http.Get(ts.URL + "/somepath")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatal("Unexpected status code", res.StatusCode)
	}
	response, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(response) != "ok" {
		t.Fatalf("expected response %q got %q", "ok", response)
	}
	wasok := <-respOK
	if !wasok {
		t.Fatal("request was nok ok")
	}
}

// Test that X-Forwarded-For is added.
func TestProxyAddForward(t *testing.T) {
	inv := newMockInventory(t, 3)
	var respOK = make(chan bool, 1)
	responder := func(req *http.Request) (*http.Response, error) {
		t.Log("X-Forwarded-For:", req.Header.Get("X-Forwarded-For"))
		respOK <- req.Header.Get("X-Forwarded-For") == "127.0.0.1"
		return httpmock.MockResponse(req)
	}
	httpmock.RegisterResponder("GET", responder)

	lb, err := NewLoadBalancer(defaultConfig.LoadBalancing, inv)
	if err != nil {
		t.Fatal(err)
	}
	conf := *defaultConfig
	conf.AddForwarded = true
	proxy := NewReverseProxyConfig(conf, lb)

	ts := httptest.NewServer(proxy)
	defer ts.Close()
	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatal("Unexpected status code", res.StatusCode)
	}
	if string(response) != "ok" {
		t.Fatalf("expected response %q got %q", "ok", response)
	}
	wasok := <-respOK
	if !wasok {
		t.Fatal("request was nok ok")
	}
}

// Test that Status code is returned.
func TestProxyStatusCode(t *testing.T) {
	inv := newMockInventory(t, 3)
	responder := func(req *http.Request) (*http.Response, error) {
		res, err := httpmock.MockResponse(req)
		res.StatusCode = 404
		return res, err
	}
	httpmock.RegisterResponder("GET", responder)

	lb, err := NewLoadBalancer(defaultConfig.LoadBalancing, inv)
	if err != nil {
		t.Fatal(err)
	}
	proxy := NewReverseProxyConfig(*defaultConfig, lb)

	ts := httptest.NewServer(proxy)
	defer ts.Close()
	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 404 {
		t.Fatal("Unexpected status code", res.StatusCode)
	}
	if string(response) != "ok" {
		t.Fatalf("expected response %q got %q", "ok", response)
	}
}

func getResponseMethod(method string) (func(req *http.Request) (*http.Response, error), chan bool) {
	checker := make(chan bool, 1)
	fn := func(req *http.Request) (*http.Response, error) {
		checker <- strings.EqualFold(method, req.Method)
		return httpmock.MockResponse(req)
	}
	return fn, checker
}

var testMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "TRACE"}

// Test that various methods make it through
func TestProxyMethods(t *testing.T) {
	inv := newMockInventory(t, 3)
	var checkers = make([]chan bool, len(testMethods))
	for i, method := range testMethods {
		fn, oker := getResponseMethod(method)
		checkers[i] = oker
		httpmock.RegisterResponder(method, fn)
	}

	lb, err := NewLoadBalancer(defaultConfig.LoadBalancing, inv)
	if err != nil {
		t.Fatal(err)
	}
	conf := *defaultConfig
	proxy := NewReverseProxyConfig(conf, lb)

	ts := httptest.NewServer(proxy)
	defer ts.Close()
	for i, method := range testMethods {
		body := bytes.NewBufferString("somebody")
		if method == "HEAD" {
			body = bytes.NewBufferString("")
		}
		req, err := http.NewRequest(method, ts.URL, body)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			if runtime.GOOS == "windows" && err.Error() == "EOF" && method == "PATCH" {
				t.Log("Let me guess. You're runnning Bitdefender as AV? ;)")
				continue
			} else {
				t.Fatal("method", method, "error:", err)
			}
		}
		if res.StatusCode != 200 {
			t.Fatal("method", method, "unexpected status code", res.StatusCode)
		}
		_, err = ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		wasok := <-checkers[i]
		if !wasok {
			t.Fatal("request for method", method, "was nok ok")
		}
	}
}

//TODO: Add Websocket tests.
