package server

import (
	"reflect"
	"testing"
	"time"
)

// Must matched parsed values of "testdata/validconfig.toml"
var valid_config = Config{
	Bind:         ":80",
	Https:        false,
	CertFile:     "cert.file",
	KeyFile:      "key.file",
	AddForwarded: true,
	WatchConfig:  false,
	LoadBalancing: LBConfig{
		Type: "roundrobin",
	},
	InventoryFile: "inventory.toml",
	Backend: BackendConfig{
		HostPrefix:    "auto-nginx",
		DialTimeout:   2000000000,
		Region:        "nyc3",
		Size:          "1gb",
		Image:         "ubuntu-14-04-x64",
		UserData:      "sample-startup.sh",
		Backups:       false,
		LatencyAvg:    30,
		HealthTimeout: 250000000,
		Token:         "878a490235d53e34b44369b8e78",
		SSHKeyID:      []string{"163420"},
	},
	Provision: ProvisionConfig{
		Enable:            true,
		MinBackends:       1,
		MaxBackends:       2,
		DownscaleLatency:  150000000,
		DownscaleTime:     900000000000,
		DownscaleEvery:    3600000000000,
		UpscaleLatency:    500000000,
		UpscaleTime:       180000000000,
		UpscaleEvery:      900000000000,
		MaxHealthFailures: 180},
}

// Test that config is read and parsed correctly
func TestReadConfig(t *testing.T) {
	s, err := NewServer("testdata/validconfig.toml")
	if err != nil {
		t.Fatal("error loading config:", err)
	}

	if !reflect.DeepEqual(s.Config, valid_config) {
		t.Fatalf("config mismatch:\nGot: %#v\nExpected: %#v", s.Config, valid_config)
	}
}

// Test that invalid syntax returns an error.
func TestReadConfigInvalid(t *testing.T) {
	_, err := NewServer("testdata/invalidsyntaxconfig.toml")
	if err == nil {
		t.Fatal("invalid syntax not reported")
	}
}

// Test that invalid value returns an error.
func TestReadConfigParam(t *testing.T) {
	_, err := NewServer("testdata/invalidconfig.toml")
	if err == nil {
		t.Fatal("invalid parameter not reported")
	}
}

// Test configuration limits are tested correctly
func TestConfigValidate(t *testing.T) {
	v := valid_config
	err := v.Validate()
	if err != nil {
		t.Fatal("valid config did not validate", err)
	}

	// Each tests a configuration variation. Last iteration should exit.
	n := 0
	for {
		// Reset config
		v = valid_config
		// Do we expect an error?
		e := true

		switch n {
		// Add test cases here:
		case 0: // "CertFile" should also be set.
			v.Https = true
			v.CertFile = ""

		case 1: // "KeyFile" should also be set.
			v.Https = true
			v.KeyFile = ""

		case 2: // Should pass.
			v.Https = true
			v.CertFile = "something"
			v.KeyFile = "something else"
			e = false

		case 3: // Ignore missing values if not useing HTTPS
			v.Https = false
			v.CertFile = ""
			v.KeyFile = ""
			e = false

		case 4: // Unset
			v.LoadBalancing.Type = ""

		case 5: // Nonexisting type
			v.LoadBalancing.Type = "dgfdgdf"

		case 6: // Wrong case
			v.LoadBalancing.Type = "RoundRobin"

		case 7: // Extra text
			v.LoadBalancing.Type = "roundrobineruitheri8tyu89"

		case 8: // Negative not allowed
			v.Backend.HealthTimeout = -1

		case 9: // 0 not allowed
			v.Backend.HealthTimeout = 0

		case 10: // Must be less than 1 second
			v.Backend.HealthTimeout = Duration(time.Second * 2)

		case 11: // Must be positive
			v.Backend.DialTimeout = -1

		case 12: // Must be bigger than 0
			v.Backend.DialTimeout = 0

		case 13: // Must be set
			v.Backend.Token = ""

		case 14: // Must be 1 or more
			v.Provision.MinBackends = 0

		case 15: // Must be less or equal to MaxBackends
			v.Provision.MinBackends = v.Provision.MaxBackends + 1

		case 16: // Equal is ok.
			v.Provision.MinBackends = v.Provision.MaxBackends
			e = false

		case 17: // Must be >= Minbackends
			v.Provision.MinBackends = 1
			v.Provision.MaxBackends = 0

		case 18: // Cannot be negative
			v.Provision.DownscaleLatency = -1

		case 19: // Cannot be 0
			v.Provision.DownscaleLatency = 0

		case 20: // Cannot be 0
			v.Provision.UpscaleLatency = 0

		case 21: // cannot be negative
			v.Provision.UpscaleLatency = -1

		case 22: // Must be > DownscaleLatency
			v.Provision.UpscaleLatency = v.Provision.DownscaleLatency

		case 23: // Must be >= 1s
			v.Provision.DownscaleTime = Duration(time.Second - time.Microsecond)

		case 24: // Must be >= 1s
			v.Provision.DownscaleTime = Duration(time.Second)
			e = false

		case 25: // Must be >= 1s
			v.Provision.DownscaleTime = -1

		case 26: // Must be >= 1s
			v.Provision.UpscaleTime = Duration(time.Second - time.Microsecond)

		case 27: // Must be >= 1s
			v.Provision.UpscaleTime = Duration(time.Second)
			e = false

		case 28: // Must be >= 1s
			v.Provision.UpscaleTime = -1

		case 29: // Must be >= 0
			v.Provision.DownscaleEvery = -1

		case 30: // Must be >= 0
			v.Provision.UpscaleEvery = -1

		case 31: // Must be > 0
			v.Provision.MaxHealthFailures = -1

		case 32: // Must be > 0
			v.Provision.MaxHealthFailures = 0

		case 33: // Must be > 0
			v.Backend.LatencyAvg = -1

		case 34: // Must be > 0
			v.Backend.LatencyAvg = 0

		case 35: // Should ignore errors if disabled.
			v.Provision.Enable = false
			v.Provision.MaxBackends = -1
			v.Provision.MinBackends = -1
			v.Provision.DownscaleEvery = -1
			v.Provision.DownscaleLatency = -1
			v.Provision.MaxHealthFailures = -1
			v.Provision.UpscaleEvery = -1
			v.Provision.UpscaleLatency = -1
			v.Provision.UpscaleTime = -1
			v.Provision.MaxHealthFailures = -1
			e = false

		case 36: // Done
			return
		default:
			t.Fatalf("test #%d not found", n)
		}
		err := v.Validate()
		if err == nil && e {
			t.Fatalf("config validation test %d did not return error, but it should", n)
		}
		if err != nil && !e {
			t.Fatalf("config validation test %d did not return error, but it returned %s", n, err)
		}
		n++
	}
}
