package server

import (
	"fmt"
	"github.com/klauspost/shutdown"
	"github.com/naoina/toml"
	"io/ioutil"
	"os"
	"sync"
)

// Inventory contains all backends in your
// inventory. This is used by the load balancer to
// select a backend to send incoming requests to.
type Inventory struct {
	backends []Backend
	bec      BackendConfig
	mu       sync.RWMutex
}

// NewInventory will a return a new Inventory
// with the supplied backends and config.
func NewInventory(b []Backend, bec BackendConfig) *Inventory {
	return &Inventory{backends: b, bec: bec}
}

// ReadInventory will read an inventory file and return the found items.
func ReadInventory(file string, bec BackendConfig) (*Inventory, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	conf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	drops := Droplets{}
	err = toml.Unmarshal(conf, &drops)
	if err != nil {
		return nil, err
	}

	inv := &Inventory{
		bec:      bec,
		backends: make([]Backend, 0, len(drops.Droplets)),
	}

	for _, v := range drops.Droplets {
		inv.backends = append(inv.backends, NewDropletBackend(v, bec))
	}

	return inv, nil
}

// SaveDroplets will save all Doplets in the current
// inventory to a specified file.
// If the file exists it will be overwritten.
func (i *Inventory) SaveDroplets(file string) error {
	// We do not want to get interrupted while saving the inventory
	if shutdown.Lock() {
		defer shutdown.Unlock()
	} else {
		return fmt.Errorf("Unable to save inventory - server is shutting down.")
	}

	// Put into object
	drops := Droplets{}
	for _, be := range i.backends {
		drop, ok := be.(*dropletBackend)
		if ok {
			drops.Droplets = append(drops.Droplets, drop.Droplet)
		}
	}

	// Marshall the inventory.
	b, err := toml.Marshal(drops)
	if err != nil {
		return err
	}

	// Finally create the file and write
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}

// Close all backends associated with this inventory.
// This will stop all stats and monitoring of the backends.
func (i *Inventory) Close() {
	i.mu.RLock()
	for _, be := range i.backends {
		be.Close()
	}
	i.mu.RUnlock()
}

// AddBackend will add a backend to the inventory
// At the moment no checks are performed, but that could
// happen in the future.
func (i *Inventory) AddBackend(be Backend) error {
	i.mu.Lock()
	i.backends = append(i.backends, be)
	i.mu.Unlock()
	return nil
}

// BackendID will return a backend with the specified ID,
// as well as a boolean indicating if it was found.
func (i *Inventory) BackendID(id string) (Backend, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, be := range i.backends {
		if be.ID() == id {
			return be, true
		}
	}
	return nil, false
}
