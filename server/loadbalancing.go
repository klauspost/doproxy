package server

import (
	"fmt"
	"log"
	"sync"
	"math"
)

// A LoadBalancer is an interface for algorithms
// that implement various methods for returning a backend.
type LoadBalancer interface {
	// Return a single backend instance.
	// If none can be found nil will be returned.
	Backend() *Backend

	// Close all backends and stop monitoring them
	Close()
}

// NewLoadBalancer returns a new load balancer described by the
// supplied configuration and inventory.
func NewLoadBalancer(conf LBConfig, i *Inventory) (LoadBalancer, error) {
	switch conf.Type {
	case "roundrobin":
		return newRoundRobin(i), nil
	case "leastconn":
		return newLeastConn(i), nil
	default:
		return nil, fmt.Errorf("Unknown load balancer type %s", conf.Type)
	}
	return nil, fmt.Errorf("NewLoadBalancer: No balancer")
}

// lbBase is common functionality for all load balancers
type lbBase struct {
	mu       sync.Mutex
	inv *Inventory
}

// roundRobin is a load balancer that simply
// switches between all the healthy backends.
type roundRobin struct {
	lbBase
	next int
}

// Close all backends in the inventory
func (r *lbBase) Close() {
	r.mu.Lock()
	r.inv.Close()
	r.mu.Unlock()
}

// NewRoundRobin Returns a new round-robin loadbalancer
func newRoundRobin(b *Inventory) LoadBalancer {
	r := &roundRobin{}
	r.inv = b
	return r
}

// Backend will return next server in a round-robin.
// Will return nil if no healthy backend can be found.
func (r *roundRobin) Backend() *Backend {
	r.mu.Lock()
	defer r.mu.Unlock()
	first := r.next
	for {
		ni := r.next % len(r.inv.backends)
		be := r.inv.backends[ni]
		r.next = ni + 1
		if be.Healthy() {
			return be
		}
		if r.next == first {
			log.Println("Unable to find a healthy backend")
			return nil
		}
	}
}

// leastConn is a load balancer that simply
// returns the backend with the fewest connections.
type leastConn struct {
	lbBase
}

// NewRoundRobin Returns a new least-connections loadbalancer
func newLeastConn(b *Inventory) LoadBalancer {
	r := &leastConn{}
	r.inv = b
	return r
}

// Backend will return the backend with the least connections
// Will return nil if no healthy backend can be found
func (r *leastConn) Backend() *Backend {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *Backend
	lowest := math.MaxInt32
	for _, be := range r.inv.backends {
		if !be.Healthy() {
			continue
		}
		conn := be.Connections()
		if conn < lowest {
			best = be
			lowest = conn
		}
	}
	if lowest == math.MaxInt32 {
		log.Println("Unable to find a healthy backend")
		return nil
	}
	return best
}


// TODO: Implement
type lowestLatency struct {
}
