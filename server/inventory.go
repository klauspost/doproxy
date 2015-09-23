package server

import (
	"fmt"
	"github.com/klauspost/shutdown"
	"github.com/naoina/toml"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

// Inventory contains all backends in your
// inventory. This is used by the load balancer to
// select a backend to send incoming requests to.
type Inventory struct {
	backends []*Backend
	bec      BackendConfig
	mu       sync.RWMutex
}

// A droplet as defined in the inventory file.
type Droplet struct {
	ID         int       `toml:"id"`
	Name       string    `toml:"name"`
	PrivateIP  string    `toml:"private-ip"`
	ServerHost string    `toml:"server-host"`
	HealthURL  string    `toml:"health-url"`
	Started    time.Time `toml:"started-time"`
}

// Droplets contains all backend droplets.
type Droplets struct {
	Droplets []Droplet `toml:"droplet"`
}

// NewInventory will create an empty inventory
func NewInventory() *Inventory {
	return &Inventory{}
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
		backends: make([]*Backend, 0, len(drops.Droplets)),
	}

	for _, v := range drops.Droplets {
		inv.backends = append(inv.backends, NewBackEnd(v, bec))
	}

	return inv, nil
}

// Save the current inventory to a specified file.
// If the file exists it will be overwritten.
func (i *Inventory) Save(file string) error {
	// We do not want to get interrupted while saving the inventory
	if shutdown.Lock() {
		defer shutdown.Unlock()
	} else {
		return fmt.Errorf("Unable to save inventory - server is shutting down.")
	}

	// Put into object
	drops := Droplets{}
	for _, be := range i.backends {
		drops.Droplets = append(drops.Droplets, be.Droplet)
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
