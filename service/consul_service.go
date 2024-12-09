package service

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"strings"
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

// GetPrometheusConfig 获取指定集群的Prometheus配置
func (c *ConsulService) GetPrometheusConfig(clusterName string) (string, error) {
	kv := c.Client.KV()
	pair, _, err := kv.Get(fmt.Sprintf("prom/cluster/%s/config", clusterName), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get prometheus config: %v", err)
	}
	if pair == nil {
		return "", nil
	}
	return string(pair.Value), nil
}

// SavePrometheusConfig 保存Prometheus配置到Consul
func (c *ConsulService) SavePrometheusConfig(clusterName string, config string) error {
	kv := c.Client.KV()
	pair := &api.KVPair{
		Key:   fmt.Sprintf("prom/cluster/%s/config", clusterName),
		Value: []byte(config),
	}
	_, err := kv.Put(pair, nil)
	if err != nil {
		return fmt.Errorf("failed to save prometheus config: %v", err)
	}
	return nil
}

// DeletePrometheusConfig 删除Prometheus配置
func (c *ConsulService) DeletePrometheusConfig(clusterName string) error {
	kv := c.Client.KV()
	_, err := kv.Delete(fmt.Sprintf("prom/cluster/%s/config", clusterName), nil)
	if err != nil {
		return fmt.Errorf("failed to delete prometheus config: %v", err)
	}
	return nil
}

// ListPrometheusClusters 列出所有Prometheus集群
func (c *ConsulService) ListPrometheusClusters() ([]string, error) {
	kv := c.Client.KV()
	pairs, _, err := kv.List("prom/cluster/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list prometheus clusters: %v", err)
	}

	clusters := make(map[string]bool)
	for _, pair := range pairs {
		parts := strings.Split(pair.Key, "/")
		if len(parts) >= 3 {
			clusters[parts[2]] = true
		}
	}

	result := make([]string, 0, len(clusters))
	for cluster := range clusters {
		result = append(result, cluster)
	}
	return result, nil
}