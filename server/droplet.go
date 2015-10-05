package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/digitalocean/godo"
	"github.com/naoina/toml"
)

// A Droplet as defined in the inventory file.
type Droplet struct {
	ID         int       `toml:"id"`
	Name       string    `toml:"name"`
	PublicIP   string    `toml:"public-ip"`
	PrivateIP  string    `toml:"private-ip"`
	ServerHost string    `toml:"server-host"`
	HealthURL  string    `toml:"health-url"`
	Started    time.Time `toml:"started-time"`
}

// Droplets contains all backend droplets.
type Droplets struct {
	Droplets []Droplet `toml:"droplet"`
}

// CreateDroplet will provision a new droplet as backend
// with the parameters given in the main configuration file.
// If no name is given, a random name with the configured prefix and
// 10 random characters will be generated.
func CreateDroplet(conf Config, name string) (*Droplet, error) {
	client := DoClient(conf.DO)

	keys := make([]godo.DropletCreateSSHKey, len(conf.DO.SSHKeyID))
	for i, key := range conf.DO.SSHKeyID {
		keys[i] = godo.DropletCreateSSHKey{ID: key}
	}

	if name == "" {
		name = conf.DO.HostPrefix + randStringRunes(10)
	}

	userdata := ""
	if conf.DO.UserData != "" {
		f, err := os.Open(conf.DO.UserData)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		buf, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		userdata = string(buf)
	}
	createRequest := &godo.DropletCreateRequest{
		Name:   name,
		Region: conf.DO.Region,
		Size:   conf.DO.Size,
		Image: godo.DropletCreateImage{
			Slug: conf.DO.Image,
		},
		Backups:           conf.DO.Backups,
		SSHKeys:           keys,
		PrivateNetworking: true,
		UserData:          userdata,
	}

	newDroplet, _, err := client.Droplets.Create(createRequest)
	if err != nil {
		return nil, err
	}

	log.Println("Droplet with ID", newDroplet.ID, "created.")

	n := 0
	for newDroplet.Status != "active" {
		log.Println("Waiting for droplet to become active.")
		time.Sleep(time.Second * 10)
		newDroplet, _, err = client.Droplets.Get(newDroplet.ID)
		if err != nil {
			return nil, err
		}
		n++
		if n == 20 {
			return nil, fmt.Errorf("Droplet did not start within 200 seconds")
		}
	}

	d, err := godoToDroplet(newDroplet)
	if err != nil {
		return nil, err
	}
	// Transfer proxy specific values
	d.ServerHost = fmt.Sprintf("%s:%d", d.PrivateIP, conf.Backend.HostPort)
	if conf.Backend.HealthHTTPS {
		d.HealthURL = fmt.Sprintf("https://%s%s", d.ServerHost, conf.Backend.HealthPath)
	} else {
		d.HealthURL = fmt.Sprintf("http://%s%s", d.ServerHost, conf.Backend.HealthPath)
	}
	return d, nil
}

func (d *Droplet) ToBackend(bec BackendConfig) (Backend, error) {
	if d.PrivateIP == "" {
		return nil, fmt.Errorf("cannot convert droplet %d to backend: no private ip v4 address", d.ID)
	}
	d.ServerHost = fmt.Sprintf("%s:%d", d.PrivateIP, bec.HostPort)
	if bec.HealthHTTPS {
		d.HealthURL = fmt.Sprintf("https://%s%s", d.ServerHost, bec.HealthPath)
	} else {
		d.HealthURL = fmt.Sprintf("http://%s%s", d.ServerHost, bec.HealthPath)
	}
	return NewDropletBackend(*d, bec), nil
}

// Delete a running droplet
func (d Droplet) Delete(conf Config) error {
	client := DoClient(conf.DO)

	resp, err := client.Droplets.Delete(d.ID)
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return ErrUnableToDelete{err: fmt.Sprintf("delete droplet returned %d, expected 204", resp.StatusCode)}
	}
	return nil
}

// Reboot a running droplet.
// Will wait up to 100 seconds or until the operation
// has been confirmed before returning.
func (d Droplet) Reboot(conf Config) error {
	client := DoClient(conf.DO)

	action, _, err := client.DropletActions.Reboot(d.ID)
	if err != nil {
		return err
	}
	n := 0
	for action.Status != "completed" {
		if action.Status == "errored" {
			return fmt.Errorf("unable to reboot droplet")
		} else if action.Status != "in-progress" {
			return fmt.Errorf("unknown action status: %s", action.Status)
		}
		// Wait a second before
		time.Sleep(time.Second)
		action, _, err = client.Actions.Get(action.ID)
		if err != nil {
			return err
		}
		n++
		if n == 100 {
			return fmt.Errorf("reboot did not complete within 100 seconds")
		}
	}
	return nil
}

func (d Droplet) String() string {
	b, err := toml.Marshal(d)
	if err != nil {
		return "Error:" + err.Error()
	}
	return string(b)
}

// DropletID returns a Droplet with the specified ID.
func (d Droplets) DropletID(id int) (drop *Droplet, ok bool) {
	for _, drop := range d.Droplets {
		if drop.ID == id {
			return &drop, true
		}
	}
	return nil, false
}

// Generate a random string of n characters.
func randStringRunes(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
