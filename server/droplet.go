package server

import (
	"fmt"
	"github.com/digitalocean/godo"
	"math/rand"
	"time"
)

// A Droplet as defined in the inventory file.
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
		UserData:          conf.DO.UserData,
	}

	newDroplet, _, err := client.Droplets.Create(createRequest)
	if err != nil {
		return nil, err
	}
	d, err := godoToDroplet(newDroplet)
	if err != nil {
		return nil, err
	}
	// Transfer proxy specific values
	d.ServerHost = fmt.Sprintf("%s:%d", d.PrivateIP, conf.Backend.HostPort)
	d.HealthURL = fmt.Sprintf("%s%s", d.ServerHost, conf.Backend.HealthPath)
	return d, nil
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
