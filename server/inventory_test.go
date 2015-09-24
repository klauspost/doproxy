package server

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Test that config is read and parsed correctly
// NOTE: "Started" is tested in TestSaveInventory
func TestReadInventory(t *testing.T) {
	inv, err := ReadInventory("testdata/validinventory.toml", BackendConfig{})
	if err != nil {
		t.Fatal("error loading inventory:", err)
	}

	bes := inv.backends
	for i, be := range bes {
		d, ok := be.(*dropletBackend)
		if !ok {
			t.Fatalf("backend type was not *dropletBackend, it was %T", be)
		}
		drop := *d
		var expect Droplet
		switch i {
		case 0:
			expect = Droplet{ID: 1, Name: "auto-nginx 1", PrivateIP: "192.168.0.1", ServerHost: "192.168.0.1:8080", HealthURL: "http://192.168.0.1:8000/index.html", Started: time.Time{}}
		case 1:
			expect = Droplet{ID: 2, Name: "auto-nginx 2", PrivateIP: "192.168.0.2", ServerHost: "192.168.0.2:8080", HealthURL: "http://192.168.0.2:8000/index.html", Started: time.Time{}}
		case 2:
			expect = Droplet{ID: -73, Name: "auto-nginx 3", PrivateIP: "192.168.0.3", ServerHost: "192.168.0.3:8080", HealthURL: "http://192.168.0.3:8000/index.html", Started: time.Time{}}
		default:
			t.Fatalf("unexpected droplet\n%#v", drop.Droplet)
		}
		if !reflect.DeepEqual(drop.Droplet, expect) {
			t.Fatalf("inventory mismatch:\nGot:n%#v\nExpected:n%#v", drop.Droplet, expect)
		}
	}
}

// Test syntax errors are reported
func TestReadInventorySyntax(t *testing.T) {
	_, err := ReadInventory("testdata/invalidsyntaxinventory.toml", BackendConfig{})
	if err == nil {
		t.Fatal("expected error loading inventory")
	}
}

// Test that config is can be read read and parsed correctly after saving
func TestSaveInventory(t *testing.T) {
	inv, err := ReadInventory("testdata/validinventory.toml", BackendConfig{})
	if err != nil {
		t.Fatal("error loading inventory:", err)
	}
	tmp := filepath.Join(os.TempDir(), "doproxy-test-inventory.toml")
	t.Log("TestSaveInventory: temporarty file at", tmp)

	// We set the time, so that is tested, rounded to 1ms.
	bes := inv.backends
	testtime := time.Now().Round(time.Millisecond)

	for _, be := range bes {
		d, ok := be.(*dropletBackend)
		if !ok {
			t.Fatalf("backend type was not *dropletBackend, it was %T", be)
		}
		d.Droplet.Started = testtime
	}
	// Save inventory
	err = inv.Save(tmp)
	if err != nil {
		t.Fatal("error writing inventory:", err)
	}

	inv, err = ReadInventory(tmp, BackendConfig{})
	if err != nil {
		t.Fatal("error re-loading inventory:", err)
	}
	bes = inv.backends
	for i, be := range bes {
		d, ok := be.(*dropletBackend)
		if !ok {
			t.Fatalf("backend type was not *dropletBackend, it was %T", be)
		}
		drop := *d

		var expect Droplet
		switch i {
		case 0:
			expect = Droplet{ID: 1, Name: "auto-nginx 1", PrivateIP: "192.168.0.1", ServerHost: "192.168.0.1:8080", HealthURL: "http://192.168.0.1:8000/index.html"}
		case 1:
			expect = Droplet{ID: 2, Name: "auto-nginx 2", PrivateIP: "192.168.0.2", ServerHost: "192.168.0.2:8080", HealthURL: "http://192.168.0.2:8000/index.html"}
		case 2:
			expect = Droplet{ID: -73, Name: "auto-nginx 3", PrivateIP: "192.168.0.3", ServerHost: "192.168.0.3:8080", HealthURL: "http://192.168.0.3:8000/index.html"}
		default:
			t.Fatalf("unexpected droplet\n%#v", drop.Droplet)
		}
		if !drop.Droplet.Started.Equal(testtime) {
			t.Fatalf("started time mismatch:got '%#v', expected: '%#v'", drop.Droplet.Started, testtime)
		}
		//  Reset the time, so we don't get a pointer mismatch in reflect.DeepEqual
		drop.Droplet.Started = time.Time{}

		if !reflect.DeepEqual(drop.Droplet, expect) {
			t.Fatalf("inventory mismatch:\nGot:n%#v\nExpected:n%#v", drop.Droplet, expect)
		}
	}
	err = os.Remove(tmp)
	if err != nil {
		t.Fatal("error removing temporary inventory file", err)
	}
}
