package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

type ReverseProxy struct {
	mu sync.RWMutex
	balancer  LoadBalancer
	conf      Config
}

// NewReverseProxy will create a new reverse
// proxy. You must set the backend and configuration
// before it is usable.
func NewReverseProxy() *ReverseProxy {
	return &ReverseProxy{}
}

// NewReverseProxyConfig will create a new reverse
// proxy with the supplied configuration and backend.
func NewReverseProxyConfig(conf Config, lb LoadBalancer) *ReverseProxy {
	return &ReverseProxy{conf: conf, balancer:lb}
}

// ServeHTTP handles reverse proxying requests.
// The function should be able to keep serving request
// and keep them alive, even if the configuration changes.
// It is ok to keep using the configuration from when the request
// was initiated for the rest of the call.
func (h *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	r.URL.Scheme = "http"
	conf := h.GetConfig()

	if conf.AddForwarded {
		// Get IP, and add it to "X-Forwarded-For".
		// This allows proxy chaining.
		if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			// If we aren't the first proxy retain prior
			// X-Forwarded-For information as a comma+space
			// separated list and fold multiple headers into one.
			if prior, ok := r.Header["X-Forwarded-For"]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
			r.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	// Override protocol, we are talking to a backend now.
	r.Proto = "HTTP/1.1"
	r.ProtoMajor = 1
	r.ProtoMinor = 1
	r.Close = false

	// Get a backend
	backend := h.GetBackend()
	if backend == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		// TODO: Add custom error message!
		fmt.Fprintf(w, "No healthy backend available :(")
		return
	}
	r.URL.Host = backend.Host()

	webSock := false
	ch := r.Header["Connection"]
	if len(ch) > 0 {
		if strings.ToLower(ch[0]) == "upgrade" {
			uh := r.Header["Upgrade"]
			if len(uh) > 0 {
				webSock = (strings.ToLower(uh[0]) == "websocket")
			}
		}
	}
	// Handle websocket upgrades
	// See https://groups.google.com/forum/#!topic/golang-nuts/KBx9pDlvFOc
	if webSock {
		hj, ok := w.(http.Hijacker)

		if !ok {
			http.Error(w, "cannot hijack writer", http.StatusInternalServerError)
			return
		}

		a, _, err := hj.Hijack()
		if err != nil {
			http.Error(w, "error hijacking websocket", http.StatusInternalServerError)
			return
		}
		defer a.Close()

		b, err := net.Dial("tcp", r.URL.Host)
		if err != nil {
			http.Error(w, "couldn't connect to backend server", http.StatusServiceUnavailable)
			return
		}
		defer b.Close()

		err = r.Write(b)
		if err != nil {
			log.Printf("writing websocket request to backend server failed: %v", err)
			http.Error(w, "writing to websocket backend failed", http.StatusInternalServerError)
			return
		}

		// Do two-way copying
		errc := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err := io.Copy(dst, src)
			errc <- err
		}
		go cp(a, b)
		go cp(b, a)

		// We return as soon as ONE direction encounter an error.
		<- errc
	} else {

		resp, err := backend.Transport().RoundTrip(r)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.Printf("Error: %v", err)
			// TODO: Add RETRY logic here!
			fmt.Fprintf(w, "Error processing request.")
			return
		}

		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		w.WriteHeader(resp.StatusCode)

		io.Copy(w, resp.Body)
		resp.Body.Close()
		copyHeader(w.Header(), resp.Trailer)
	}
}

// Copied from
// https://github.com/golang/go/blob/release-branch.go1.5/src/net/http/httputil/reverseproxy.go#L82
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Replace the configuration with another one.
func (h* ReverseProxy) SetConfig(conf Config) {
	h.mu.Lock()
	h.conf = conf
	h.mu.Unlock()
}

// SetBackends will replace the current backends
// with the new ones. Requests currently being served will
// still go to the old backends, but new ones will go to
// a new one.
func (h* ReverseProxy) SetBackends(balancer LoadBalancer) {
	h.mu.Lock()
	if h.balancer != nil {
		h.balancer.Close()
	}
	h.balancer = balancer
	h.mu.Unlock()
}

// GetConfig will return a copy of the latest configuration.
func (h *ReverseProxy) GetConfig() Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conf
}

// GetBackend will return a backend from
// the current load balancer.
func (h *ReverseProxy) GetBackend() Backend {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.balancer.Backend()
}

