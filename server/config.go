package server

import (
	"fmt"
	"github.com/naoina/toml"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

// Config contains the main server configuration
// This maps directly to the main config file.
type Config struct {
	Bind          string          `toml:"bind"`
	Https         bool            `toml:"https"`
	CertFile      string          `toml:"tls-cert-file"`
	KeyFile       string          `toml:"tls-key-file"`
	AddForwarded  bool            `toml:"add-x-forwarded-for"`
	WatchConfig   bool            `toml:"watch-config"` // Watch the configuration file for changes
	LoadBalancing LBConfig        `toml:"loadbalancing"`
	InventoryFile string          `toml:"inventory-file"`
	Backend       BackendConfig   `toml:"backend"`
	Provision     ProvisionConfig `toml:"provisioning"`
	DO            DOConfig        `toml:"do-provisioner"`
}

func ReadConfigFile(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	conf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	config := Config{}
	err = toml.Unmarshal(conf, &config)
	if err != nil {
		return nil, err
	}

	err = config.Validate()
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ReadConfig will open the file with the supplied name
// and read the configuration from that.
// Use init, to initialize the configuration on startup, if
// you are reloading the configuration set it to false.
// If successful, the new config will be applied to the server.
func (s *Server) ReadConfig(file string, init bool) error {
	config, err := ReadConfigFile(file)
	if err != nil {
		return err
	}
	if init {
		s.mu.Lock()
		s.Config = *config
		s.mu.Unlock()
		return nil
	}
	err = s.UpdateConfig(*config)
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
	err = c.Provision.Validate()
	if err != nil {
		return err
	}
	err = c.DO.Validate()
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
// instantiate and destroy backends on demand.
type BackendConfig struct {
	DialTimeout   Duration `toml:"dial-timeout"`            // Timeout for connecting to a backend.
	LatencyAvg    int      `toml:"latency-average-seconds"` // Measure latency over this many seconds
	HealthTimeout Duration `toml:"health-check-timeout"`    // Timeout for a health check. Should be less than 1 second.
	HostPort      int      `toml:"new-host-port"`           // Host port the proxy should connect to.
	HealthPath    string   `toml:"new-host-health-path"`    // Health path to use.
	HealthHTTPS   bool     `toml:"new-host-health-https"`   // Set to true if the health check on new backs is https.
	DisableHealth bool     `toml:"disable-health-check"`    // Disable health checks.
}

// Validate backend configuration.
// Will return the first error found.
// FIXME: Check remaining settings.
func (c BackendConfig) Validate() error {
	if c.HealthTimeout <= 0 {
		return fmt.Errorf("'health-check-timeout' = '%s' cannot be 0 or negative", c.HealthTimeout)
	}
	if c.HealthTimeout > Duration(time.Second) {
		return fmt.Errorf("'health-check-timeout' = '%s' cannot be longer than '1s'", c.HealthTimeout)
	}
	if c.DialTimeout <= 0 {
		return fmt.Errorf("'dial-timeout' = '%s' cannot be 0 or negative", c.DialTimeout)
	}
	if c.LatencyAvg <= 0 {
		return fmt.Errorf("'latency-average-seconds' = '%d' cannot be 0 or negative", c.LatencyAvg)
	}
	return nil
}

// DigitalOcean provisioning config
type DOConfig struct {
	Enable     bool   `toml:"enable"`
	HostPrefix string `toml:"hostname-prefix"`
	Region     string `toml:"region"`
	Size       string `toml:"size"`
	Image      string `toml:"image"`
	UserData   string `toml:"user-data"`
	Backups    bool   `toml:"backups"`
	Token      string `toml:"token"`
	SSHKeyID   []int  `toml:"ssh-key-ids"`
}

func (c DOConfig) Validate() error {
	if !c.Enable {
		return nil
	}
	if c.Token == "" {
		return fmt.Errorf("No 'token' specified")
	}
	return nil
}

// ProvisionConfig contains configuration for starting
// and stopping backends. This information is mainly used to
// instantiate and destroy backends on demand.
type ProvisionConfig struct {
	Enable bool `toml:"enable"`

	// The minimum number of running backends.
	MinBackends int `toml:"min-backends"`
	// The maximum number of running backends.
	MaxBackends int `toml:"max-backends"`

	// If latency is below this, deprovision one server.
	DownscaleLatency Duration `toml:"downscale-latency"`
	// How long should the latency be below threshold before a server is deprovisioned.
	// This is an Exponentially Weighted Moving Average.
	DownscaleTime Duration `toml:"downscale-time"`
	// How long between a server can be deprovisioned.
	DownscaleEvery Duration `toml:"downscale-every"`

	// If the latency is above this, provision a new server.
	UpscaleLatency Duration `toml:"upscale-latency"`
	// How long should the latency be below threshold before a server is provisioned.
	// This is an Exponentially Weighted Moving Average.
	UpscaleTime Duration `toml:"upscale-time"`
	// How long between a new server can be provisioned.
	UpscaleEvery Duration `toml:"upscale-every"`

	// If a server fails this many health consequtive health checks, it will be deprovisioned.
	// Health checks is performed every second.
	MaxHealthFailures int `toml:"max-health-failures"`
}

// Validate provisioning configuration.
// Will return the first error found.
func (c ProvisionConfig) Validate() error {
	// We skip more checks if not enabled.
	if !c.Enable {
		return nil
	}
	if c.MinBackends < 1 {
		return fmt.Errorf("provisioning: 'min-backends' cannot be less than 1")
	}
	if c.MaxBackends < c.MinBackends {
		return fmt.Errorf("provisioning: 'max-backends' cannot be less 'min-backends'")
	}
	if c.DownscaleLatency <= 0 {
		return fmt.Errorf("provisioning: 'downscale-latency' cannot be less than 0 or negative")
	}
	if c.UpscaleLatency <= c.DownscaleLatency {
		return fmt.Errorf("provisioning: 'upscale-latency' cannot be less than or equal to 'downscale latency'")
	}
	if c.DownscaleTime < Duration(time.Second) {
		return fmt.Errorf("provisioning: 'downscale-time' cannot be less 1 second")
	}
	if c.UpscaleTime < Duration(time.Second) {
		return fmt.Errorf("provisioning: 'upscale-time' cannot be less 1 second")
	}
	if c.DownscaleEvery < 0 {
		return fmt.Errorf("provisioning: 'downscale-every' cannot be negative")
	}
	if c.UpscaleEvery < 0 {
		return fmt.Errorf("provisioning: 'upscale-every' cannot be negative")
	}
	if c.MaxHealthFailures < 1 {
		return fmt.Errorf("provisioning: 'max-health-failures' must be bigger than 0")
	}
	return nil
}

// Duration is our own time.Duration that fulfills  the
// toml.UnmarshalTOML interface.
type Duration time.Duration

func (d *Duration) UnmarshalTOML(data []byte) error {
	dur, err := time.ParseDuration(strings.Trim(string(data), "\""))
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) String() string {
	return time.Duration(d).String()
}
