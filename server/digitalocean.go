package server

import (
	"fmt"
	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"log"
	"time"
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

// godoToDroplet transfers a DO API object to an internal representation
func godoToDroplet(do *godo.Droplet) (*Droplet, error) {
	net, err := godoGetPrivateV4(do.Networks)
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
		ID:        do.ID,
		Name:      do.Name,
		PrivateIP: net.IPAddress,
		Started:   started,
	}
	return &drop, nil
}

// godoGetPrivateV4 will return the first private V4 network
// interface from a collection of network interfaces. If there is
// no private v4 network interface an error will be returned.
func godoGetPrivateV4(net *godo.Networks) (*godo.NetworkV4, error) {
	if net == nil {
		return nil, fmt.Errorf("no network info returned")
	}
	for _, ni := range net.V4 {
		if ni.Type == "private" {
			return &ni, nil
		}
	}
	return nil, fmt.Errorf("no private ipv4 interfaces found")
}
