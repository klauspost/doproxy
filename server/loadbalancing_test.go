package server

import (
	"testing"
)

func TestRoundRobin(t *testing.T) {
	conf := LBConfig{Type: "roundrobin"}
	inv := newMockInventory(t, 5)

	lb, err := NewLoadBalancer(conf, inv)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(inv.backends)*5; i++ {
		be := lb.Backend()
		if be == nil {
			t.Fatal("got no backend on iteration", i)
		}
		mbe := be.(*mockBackend)
		expectn := i % len(inv.backends)
		if mbe.n != expectn {
			t.Fatal("iteration", i, "expected backend ", expectn, "got", mbe.n)
		}
	}
	// Mark one as unhealthy
	mark := inv.backends[2].(*mockBackend)
	mark.Stats.mu.Lock()
	mark.Stats.Healthy = false
	mark.Stats.mu.Unlock()
	for i := 0; i < len(inv.backends)*5; i++ {
		be := lb.Backend()
		if be == nil {
			t.Fatal("got no backend on iteration", i)
		}
		mbe := be.(*mockBackend)
		expectn := i % len(inv.backends)
		// Number 2 is unhealthy, so it should select the next
		if expectn == 2 {
			expectn = 3
			i++
		}
		if mbe.n != expectn {
			t.Fatal("iteration", i, "expected backend ", expectn, "got", mbe.n)
		}
	}
	// Mark all unhealthy
	for i := 0; i < len(inv.backends); i++ {
		mark := inv.backends[i].(*mockBackend)
		mark.Stats.mu.Lock()
		mark.Stats.Healthy = false
		mark.Stats.mu.Unlock()
	}
	be := lb.Backend()
	if be != nil {
		t.Fatal("all backends should be unhealthy, but got one anyway")
	}
}
