package server

import (
	"fmt"
	"github.com/naoina/toml"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// Config contains the main server configuration
// This maps directly to the main config file.
type Config struct {
	Bind          string        `toml:"bind"`
	Https         bool          `toml:"https"`
	CertFile      string        `toml:"tls-cert-file"`
	KeyFile       string        `toml:"tls-key-file"`
	AddForwarded  bool          `toml:"add-x-forwarded-for"`
	WatchConfig   bool          `toml:"watch-config"` // Watch the configuration file for changes
	LoadBalancing LBConfig      `toml:"loadbalancing"`
	InventoryFile string        `toml:"inventory-file"`
	Backend       BackendConfig `toml:"backend"`
}

// ReadConfig will open the file with the supplied name
// and read the configuration from that.
// Use init, to initialize the configuration on startup, if
// you are reloading the configuration set it to false.
// If successful, the new config will be applied to the server.
func (s *Server) ReadConfig(file string, init bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	conf, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	config := Config{}
	err = toml.Unmarshal(conf, &config)
	if err != nil {
		return err
	}

	err = config.Validate()
	if err != nil {
		return err
	}

	if init {
		s.mu.Lock()
		s.Config = config
		s.mu.Unlock()
		return nil
	}
	err = s.UpdateConfig(config)
	if err != nil {
		return err
	}
	log.Println("Loaded configuration", file)

	return nil
}

// UpdateConfig will check and apply a revised config.
// If the new config results in an error, the old config will remain.
func (s *Server) UpdateConfig(new Config) (err error) {
	s.mu.Lock()
	old := s.Config

	// If error has been set, revert the configuration.
	defer func() {
		if err != nil {
			s.Config = old
		}
		s.mu.Unlock()
	}()
	if old.WatchConfig != new.WatchConfig {
		return fmt.Errorf("cannot modify 'watch-config' while server is running. restart to apply.")
	}
	if old.Bind != new.Bind {
		return fmt.Errorf("cannot modify 'bind' while server is running. restart to apply.")
	}
	if old.Https != new.Https {
		return fmt.Errorf("cannot modify 'https' while server is running. restart to apply.")
	}
	if old.CertFile != new.CertFile {
		return fmt.Errorf("cannot modify 'tls-certfile' while server is running. restart to apply.")
	}
	if old.KeyFile != new.KeyFile {
		return fmt.Errorf("cannot modify 'tls-keyfile' while server is running. restart to apply.")
	}
	// New inventory file.
	var newLB LoadBalancer
	if old.InventoryFile != new.InventoryFile {
		inv, err := ReadInventory(new.InventoryFile, new.Backend)
		if err != nil {
			return err
		}
		newLB, err = NewLoadBalancer(s.Config.LoadBalancing, inv)
		if err != nil {
			return err
		}
	}
	s.handler.SetBackends(newLB)
	s.handler.SetConfig(new)
	s.Config = new
	return
}

// Validate if settings in configuration are valid.
// The function will validate all subobjects as well.
// Will return an error with the first problem found.
func (c Config) Validate() error {
	if c.Https && c.CertFile == "" {
		return fmt.Errorf("HTTPS requested, but no 'tls-cert-file' specified")
	}
	if c.Https && c.KeyFile == "" {
		return fmt.Errorf("HTTPS requested, but no 'tls-key-file' specified")
	}
	err := c.LoadBalancing.Validate()
	if err != nil {
		return err
	}
	err = c.Backend.Validate()
	if err != nil {
		return err
	}
	return nil
}

// LBConfig contains settings for the load balancer.
type LBConfig struct {
	Type string `toml:"type"`
}

// Validate if settings in the load balancer configuration
// are valid.
func (c LBConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("loadbalancing: No 'type' specified")
	}
	_, err := NewLoadBalancer(c, nil)
	if err != nil {
		return err
	}
	return nil
}

// BackendConfig contains configuration for handling
// backends. This information is mainly used to
// instanciate and destroy backends on demand.
type BackendConfig struct {
	HostPrefix    string   `toml:"hostname-prefix"`
	DialTimeout   string   `toml:"dial-timeout"`
	Region        string   `toml:"region"`
	Size          string   `toml:"size"`
	Image         string   `toml:"image"`
	UserData      string   `toml:"user-data"`
	Backups       bool     `toml:"backups"`
	LatencyAvg    int      `toml:"latency-average-seconds"`
	HealthTimeout string   `toml:"health-check-timeout"`
	Token         string   `toml:"token"`
	SSHKeyID      []string `toml:"ssh-key-ids"`
}

// Validate backend configuration.
// Will return the first error found.
// FIXME: Check remaining settings.
func (c BackendConfig) Validate() error {
	to, err := time.ParseDuration(c.HealthTimeout)
	if err != nil {
		return fmt.Errorf("Cannot convert 'health-check-timeout' = '%s' to time.Duration: %s", c.HealthTimeout, err.Error())
	}
	if to <= 0 {
		return fmt.Errorf("'health-check-timeout' = '%s' cannot be 0 or negative", c.HealthTimeout)
	}
	if to > time.Second {
		return fmt.Errorf("'health-check-timeout' = '%s' cannot be longer than '1s'", c.HealthTimeout)
	}
	to, err = time.ParseDuration(c.DialTimeout)
	if err != nil {
		return fmt.Errorf("Cannot convert 'dial-timeout' = '%s' to time.Duration: %s", c.DialTimeout, err.Error())
	}
	if to <= 0 {
		return fmt.Errorf("'dial-timeout' = '%s' cannot be 0 or negative", c.DialTimeout)
	}
	if c.Token == "" {
		return fmt.Errorf("No 'token' specified")
	}
	return nil
}
