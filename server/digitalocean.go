package server

import (
	"fmt"
	"log"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

func DoClient(conf DOConfig) *godo.Client {
	token := &oauth2.Token{AccessToken: conf.Token}
	t := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(oauth2.NoContext, t)
	return godo.NewClient(oauthClient)
}

type ErrUnableToDelete struct {
	err string
}

func (e ErrUnableToDelete) Error() string {
	return e.err
}

// RemoveDroplet will query DO to remove a droplet.
// The ID of the droplet is used to identify the droplet.
// If an error is returned the droplet most likely has not been removed.
func RemoveDroplet(conf Config, drop Droplet) error {
	client := DoClient(conf.DO)

	resp, err := client.Droplets.Delete(drop.ID)
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return ErrUnableToDelete{err: fmt.Sprintf("delete droplet returned %d, expected 204", resp.StatusCode)}
	}
	return nil
}

// ListDroplets list all droplets currently running.
func ListDroplets(conf Config) (*Droplets, error) {
	client := DoClient(conf.DO)

	d, _, err := client.Droplets.List(nil)
	if err != nil {
		return nil, err
	}
	var drops []Droplet
	for _, drop := range d {
		d, err := godoToDroplet(&drop)
		if err != nil {
			return nil, err
		}
		drops = append(drops, *d)
	}
	return &Droplets{drops}, nil
}

// godoToDroplet transfers a DO API object to an internal representation
func godoToDroplet(do *godo.Droplet) (*Droplet, error) {
	pub, priv, err := godoNetV4(do.Networks)
	if err != nil {
		return nil, err
	}
	started, err := time.Parse(time.RFC3339, do.Created)
	if err != nil {
		log.Println("Error converting creation time:", err)
		log.Println("Setting creation time to servber time.")
		started = time.Now()
	}
	drop := Droplet{
		ID:      do.ID,
		Name:    do.Name,
		Started: started,
	}
	if pub != nil {
		drop.PublicIP = pub.IPAddress
	}
	if priv != nil {
		drop.PrivateIP = priv.IPAddress
	}
	return &drop, nil
}

// godoNetV4 will return the first public and private V4 network
// interface from a collection of network interfaces. If there is
// no public and private v4 network interface an error will be returned.
func godoNetV4(net *godo.Networks) (pub *godo.NetworkV4, priv *godo.NetworkV4, err error) {
	if net == nil {
		return nil, nil, fmt.Errorf("no network info")
	}
	for i, ni := range net.V4 {
		switch {
		case ni.Type == "private" && priv == nil:
			priv = &net.V4[i]

		case ni.Type == "public" && pub == nil:
			pub = &net.V4[i]
		}
	}
	if pub == nil && priv == nil {
		return nil, nil, fmt.Errorf("unable to find any ipv4 network interfaces")
	}
	return
}
