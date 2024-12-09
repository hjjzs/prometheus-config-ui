package service

import (
	"fmt"
	"github.com/hashicorp/consul/api"
)

type ConsulService struct {
	Client *api.Client
}

func NewConsulService(address string, token string) (*ConsulService, error) {
	config := api.DefaultConfig()
	config.Address = address
	config.Token = token
	
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %v", err)
	}
	
	return &ConsulService{Client: client}, nil
}

