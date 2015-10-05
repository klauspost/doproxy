package server

type Provisioner interface {
	Add() error
	Remove() error
}

type provisioner struct {
	Config ProvisionConfig
}

func newProvisioner(c ProvisionConfig, lb LoadBalancer) (*provisioner, error) {
	p := provisioner{Config: c}
	return &p, nil
}
