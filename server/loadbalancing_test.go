package server

import (
	"testing"
)

func TestRoundRobin(t *testing.T) {
	conf := LBConfig{Type: "roundrobin"}
	inv := newMockInventory(t, 5)
	defer inv.Close()

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

type leastConnTest struct {
	conns     []int // Connection numbers to simulate
	expect    []int // Which results (indexes into conns) are allowed
	unhealthy []int // Which backends should be marked unhealthy
}

var leastConnTests = []leastConnTest{
	leastConnTest{conns: []int{1, 0, 0, 0}, expect: []int{1, 2, 3}},
	leastConnTest{conns: []int{1, 2, 3, 4}, expect: []int{0}},
	leastConnTest{conns: []int{4, 3, 2, 1}, expect: []int{3}},
	leastConnTest{conns: []int{0, 1, 1, 1}, expect: []int{0}},
	leastConnTest{conns: []int{1, 0, 1, 1}, expect: []int{1}},
	leastConnTest{conns: []int{1, 1, 0, 1}, expect: []int{2}},
	leastConnTest{conns: []int{1, 1, 1, 0}, expect: []int{3}},
	leastConnTest{conns: []int{1000}, expect: []int{0}},
	leastConnTest{conns: []int{0}, expect: []int{0}},
	leastConnTest{conns: []int{5000, 4000, 3000, 2000, 1000, 100, 50}, expect: []int{6}},

	leastConnTest{conns: []int{1, 0, 0, 0}, expect: []int{1}, unhealthy: []int{2, 3}},
	leastConnTest{conns: []int{1, 2, 3, 4}, expect: []int{0}, unhealthy: []int{2, 3}},
	leastConnTest{conns: []int{4, 3, 2, 1}, expect: []int{1}, unhealthy: []int{2, 3}},
	leastConnTest{conns: []int{0, 1, 1, 1}, expect: []int{1, 2, 3}, unhealthy: []int{0}},
	leastConnTest{conns: []int{1, 0, 1, 1}, expect: []int{2, 3}, unhealthy: []int{0, 1}},
	leastConnTest{conns: []int{1, 1, 0, 1}, expect: []int{2}, unhealthy: []int{0, 1}},
	leastConnTest{conns: []int{1, 1, 1, 0}, expect: []int{3}, unhealthy: []int{0, 1}},
	leastConnTest{conns: []int{1000}, expect: []int{}, unhealthy: []int{0}},
	leastConnTest{conns: []int{5000, 4000, 3000, 2000, 1000, 100, 25}, expect: []int{4}, unhealthy: []int{6, 5}},
	leastConnTest{conns: []int{50, 4000, 3000, 2000, 1000, 100, 25}, expect: []int{0}, unhealthy: []int{6, 5}},
}

func TestLeastConn(t *testing.T) {
	conf := LBConfig{Type: "leastconn"}
	for i, test := range leastConnTests {
		inv := newMockInventory(t, len(test.conns))
		lb, err := NewLoadBalancer(conf, inv)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range test.unhealthy {
			mark := inv.backends[n].(*mockBackend)
			mark.backend.Close() // Close the monitor, so it doesn't interfere.
			mark.Stats.mu.Lock()
			mark.Stats.Healthy = false
			mark.Stats.mu.Unlock()
			healthy := inv.backends[n].Healthy()
			if healthy {
				t.Fatal("test", i, "Healthy was not false")
			}
		}
		for n, num := range test.conns {
			mark := inv.backends[n].(*mockBackend)
			mark.rt.mu.Lock()
			mark.rt.running = num
			mark.rt.mu.Unlock()
			connections := inv.backends[n].Connections()
			if connections != num {
				t.Fatal("test", i, "Connections was not set to", num, "got", connections)
			}
		}
		be := lb.Backend()
		if len(test.expect) == 0 {
			if be != nil {
				t.Fatal("test", i, "did not expect any backends, but got number", be)
			}
			continue
		}
		mbe := be.(*mockBackend)
		found := false
		for _, n := range test.expect {
			if mbe.n == n {
				found = true
			}
		}
		if !found {
			t.Fatal("test", i, "unexpected backend. Got", mbe.n, "expected one of", test.expect)
		}
		inv.Close()
	}
}
