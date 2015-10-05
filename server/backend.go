package server

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/klauspost/shutdown"
)

// A Backend is a single running backend instance.
// It will monitor itself and update health and stats every second.
type Backend interface {
	Transport() http.RoundTripper // Returns a transport for the backend
	ID() string                   // A string identifier of this specific backend
	Name() string                 // A name for this backend
	Host() string                 // Returns the hostname of the backend
	Healthy() bool                // Is the backend healthy?
	Statistics() *Stats           // Returns a copy of the latest statistics. Updated every second.
	Connections() int             // Return the current number of connections
	Close()                       // Close the backend (before shutdown/reload).
}

// backend is a common base used for sharing functionality
// between different backend types, so implementing different
// ones are easier.
type backend struct {
	rt           *statRT
	healthClient *http.Client
	closeMonitor chan chan struct{}
	Stats        Stats
	ServerHost   string
	HealthURL    string
}

// newBackend returns a new generic backend.
// It will start monitoring the backend at once
func newBackend(bec BackendConfig, serverHost, healthURL string) *backend {
	b := &backend{
		ServerHost: serverHost,
		HealthURL:  healthURL,
	}
	// Create a transport that is used for health checks.
	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   time.Duration(bec.HealthTimeout),
			KeepAlive: 0,
		}).Dial,
		DisableKeepAlives:  true,
		DisableCompression: true,
	}
	b.healthClient = &http.Client{Transport: tr}

	// Reset running stats.
	b.Stats.Latency = ewma.NewMovingAverage(float64(bec.LatencyAvg))
	b.Stats.FailureRate = ewma.NewMovingAverage(10)

	// Set up the backend transport.
	tr = &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Duration(bec.DialTimeout))
		},
		Proxy: http.ProxyFromEnvironment,
	}
	b.rt = newStatTP(tr)

	// If we have no health url, assume healthy
	if healthURL == "" {
		b.Stats.Healthy = true
	}

	if !bec.DisableHealth {
		b.closeMonitor = make(chan chan struct{}, 0)
		go b.startMonitor()
	}
	return b
}

// startMonitor will monitor stats of the backend
// Will at times require BOTH rt and Stats mutex.
// This means that no other goroutine should acquire
// both at the same time.
func (b *backend) startMonitor() {
	s := b.rt
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	exit := shutdown.First()
	end := b.closeMonitor
	previous := time.Now()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Now().Sub(previous)
			previous = time.Now()
			s.mu.Lock()
			b.Stats.mu.Lock()
			if s.requests == 0 {
				b.Stats.Latency.Add(0)
				b.Stats.FailureRate.Add(0)
			} else {
				b.Stats.Latency.Add(float64(s.latencySum) / float64(elapsed) / float64(s.requests))
				b.Stats.FailureRate.Add(float64(s.errors) / float64(s.requests))
			}
			s.requests = 0
			s.errors = 0
			s.latencySum = 0
			s.mu.Unlock()

			// Perform health check
			b.healthCheck()

			if b.Stats.Healthy && b.Stats.healthFailures > 5 {
				log.Println("5 Consequtive health tests failed. Marking as unhealty.")
				b.Stats.Healthy = false
			}
			if !b.Stats.Healthy && b.Stats.healthFailures == 0 {
				log.Println("Health check succeeded. Marking as healty")
				b.Stats.Healthy = true
			}
			b.Stats.mu.Unlock()
		case n := <-end:
			exit.Cancel()
			close(n)
			return
		case n := <-exit:
			close(n)
			return
		}
	}
}

// healthCheck will check the health by connecting
// to the healthURL of the backend.
// This is called by healthCheck every second.
// It assumes b.Stats.mu is locked, but will unlock it while
// the request is running.
func (b *backend) healthCheck() {
	// If no checkurl har been set, assume we are healthy
	if b.HealthURL == "" {
		b.Stats.Healthy = true
		return
	}

	req, err := http.NewRequest("GET", b.HealthURL, nil)
	if err != nil {
		log.Println("Error checking health of", b.HealthURL, "Error:", err)
	}

	req.Header.Set("User-Agent", "doproxy health checker")

	b.Stats.mu.Unlock()
	// Perform the check
	resp, err := b.healthClient.Do(req)

	b.Stats.mu.Lock()
	// Check response
	if err != nil {
		b.Stats.healthFailures++
		log.Println("Error checking health of", b.HealthURL, "Error:", err)
		return
	}
	if resp.StatusCode >= 500 {
		b.Stats.healthFailures++
		log.Println("Error checking health of", b.HealthURL, "Status code:", resp.StatusCode)
	} else {
		// Reset failures
		b.Stats.healthFailures = 0
	}
	resp.Body.Close()
}

// Transport returns a RoundTripper that will collect stats
// about the backend.
func (b *backend) Transport() http.RoundTripper {
	return b.rt
}

// Healthy returns the healthy state of the backend
func (b *backend) Healthy() bool {
	b.Stats.mu.RLock()
	ok := b.Stats.Healthy
	b.Stats.mu.RUnlock()
	return ok
}

// Healthy returns the healthy state of the backend
func (b *backend) Statistics() *Stats {
	b.Stats.mu.RLock()
	s := b.Stats
	b.Stats.mu.RUnlock()
	return &s
}

// Host returns the host address of the backend.
func (b *backend) Host() string {
	return b.ServerHost
}

// Close the backend, which will shut down monitoring
// of the backend.
func (b *backend) Close() {
	if b.closeMonitor == nil {
		return
	}
	c := make(chan struct{})
	b.closeMonitor <- c
	<-c
	close(b.closeMonitor)
	b.closeMonitor = nil
}

// Connections returns the number of currently running requests.
// Does not include websocket connections.
func (b *backend) Connections() int {
	b.rt.mu.RLock()
	n := b.rt.running
	b.rt.mu.RUnlock()
	return n
}

func (s *statRT) RoundTrip(req *http.Request) (*http.Response, error) {
	// Record this request as running
	s.mu.Lock()
	s.running++
	s.mu.Unlock()

	// Time the request roundtrip time
	start := time.Now()
	resp, err := s.rt.RoundTrip(req)
	dur := start.Sub(time.Now())

	// Update stats
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running--
	s.requests++
	s.latencySum += dur
	if err != nil {
		s.errors++
		return nil, err
	}
	// Any status code above or equal to 500 is recorded as an error.
	if resp.StatusCode >= 500 {
		s.errors++
		return resp, nil
	}
	return resp, nil
}

// Stats contain regularly updated statistics about a
// backend. To access be sure to hold the 'mu' mutex.
type Stats struct {
	mu             sync.RWMutex
	healthFailures int // Number of total health check failures
	Healthy        bool
	Latency        ewma.MovingAverage
	FailureRate    ewma.MovingAverage
}

// statRT wraps a http.RoundTripper around statistics that can
// be used for load balancing.
type statRT struct {
	rt         http.RoundTripper
	mu         sync.RWMutex
	latencySum time.Duration
	running    int
	requests   int
	errors     int
}

// dropletBackend is a a backend instance with a DigitalOcean droplet
// behind it.
type DropletBackend struct {
	*backend
	Droplet Droplet
}

// NewDropletBackend returns a Backend configured with the
// Droplet information.
func NewDropletBackend(d Droplet, bec BackendConfig) Backend {
	b := &DropletBackend{
		backend: newBackend(bec, d.ServerHost, d.HealthURL),
		Droplet: d,
	}
	return b
}

// ID returns a unique ID of this backend
func (d *DropletBackend) ID() string {
	return strconv.Itoa(d.Droplet.ID)
}

// ID returns a name of this backend
func (d *DropletBackend) Name() string {
	return d.Droplet.Name
}

func newStatTP(rt http.RoundTripper) *statRT {
	s := &statRT{rt: rt}
	return s
}
