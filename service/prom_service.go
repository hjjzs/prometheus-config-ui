package service

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"consul-ui/types"
	"strings"
	promconfig "github.com/prometheus/prometheus/config"
	"gopkg.in/yaml.v3"
)

type PromService struct {
	consul *ConsulService
}

func NewPromService(consul *ConsulService) *PromService {
	return &PromService{
		consul: consul,
	}
}

// 列出所有集群
func (s *PromService) ListClusters() ([]types.Cluster, error) {
	pairs, _, err := s.consul.Client.KV().List("prom/cluster/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %v", err)
	}

	clusterMap := make(map[string]bool)
	clusters := make([]types.Cluster, 0)
	
	for _, pair := range pairs {
		if name := getClusterName(pair.Key); name != "" {
			if !clusterMap[name] {
				clusterMap[name] = true
				clusters = append(clusters, types.Cluster{
					Name: name,
				})
			}
		}
	}
	
	return clusters, nil
}

// 获取配置
func (s *PromService) GetConfig(clusterName string) (string, error) {
	pair, _, err := s.consul.Client.KV().Get(fmt.Sprintf("prom/cluster/%s/config", clusterName), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get config: %v", err)
	}
	if pair == nil {
		return "", nil
	}
	return string(pair.Value), nil
}

// 保存配置
func (s *PromService) SaveConfig(clusterName, configContent string) error {
	var cfg promconfig.Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		return fmt.Errorf("invalid config format: %v", err)
	}

	// 使用默认值进行验证
	// if _, err := promconfig.Load(configContent, false, nil); err != nil {
	// 	return fmt.Errorf("invalid prometheus config: %v", err)
	// }

	// 保存到 Consul
	key := fmt.Sprintf("prom/cluster/%s/config", clusterName)
	_, err := s.consul.Client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(configContent),
	}, nil)
	return err
}

// 添加新集群
func (s *PromService) AddCluster(name string) error {
	key := fmt.Sprintf("prom/cluster/%s/config", name)
	_, err := s.consul.Client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(""),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to add cluster: %v", err)
	}
	return nil
}

// 删除集群
func (s *PromService) DeleteCluster(name string) error {
	prefix := fmt.Sprintf("prom/cluster/%s", name)
	_, err := s.consul.Client.KV().DeleteTree(prefix, nil)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %v", err)
	}
	return nil
}

// 辅助函数
func getClusterName(key string) string {
	// 路径格式: prom/cluster/name/... 或 prom/cluster/name
	parts := strings.Split(key, "/")
	if len(parts) >= 3 && parts[0] == "prom" && parts[1] == "cluster" {
		return parts[2]
	}
	return ""
} 