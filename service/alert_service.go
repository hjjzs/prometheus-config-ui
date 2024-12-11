package service

import (
    "fmt"
    "github.com/hashicorp/consul/api"
    "consul-ui/types"
    "strings"
	"gopkg.in/yaml.v3"
	alertconfig "github.com/prometheus/alertmanager/config"
)

type AlertService struct {
    consul *ConsulService
}

func NewAlertService(consul *ConsulService) *AlertService {
    return &AlertService{
        consul: consul,
    }
}

// 列出所有集群
func (s *AlertService) ListClusters() ([]types.Cluster, error) {
    pairs, _, err := s.consul.Client.KV().List("alert/cluster/", nil)
    if err != nil {
        return nil, fmt.Errorf("failed to list clusters: %v", err)
    }

    clusterMap := make(map[string]bool)
    clusters := make([]types.Cluster, 0)
    
    for _, pair := range pairs {
        if name := getAlertClusterName(pair.Key); name != "" {
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
func (s *AlertService) GetConfig(clusterName string) (string, error) {
    pair, _, err := s.consul.Client.KV().Get(fmt.Sprintf("alert/cluster/%s/config", clusterName), nil)
    if err != nil {
        return "", fmt.Errorf("failed to get config: %v", err)
    }
    if pair == nil {
        return "", nil
    }
    return string(pair.Value), nil
}

// 保存配置
func (s *AlertService) SaveConfig(clusterName, configContent string) error {
		// TODO: 添加alertmanager配置验证
	var cfg alertconfig.Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		return fmt.Errorf("invalid alertmanager config format: %v", err)
	}

	// 检查必需的顶级配置项
	if cfg.Global == nil {
		return fmt.Errorf("missing required 'global' section in alertmanager config")
	}
	if cfg.Route == nil {
		return fmt.Errorf("missing required 'route' section in alertmanager config") 
	}
	if len(cfg.Receivers) == 0 {
		return fmt.Errorf("missing required 'receivers' section in alertmanager config")
	}
	
    // 保存到 Consul
    key := fmt.Sprintf("alert/cluster/%s/config", clusterName)
    _, err := s.consul.Client.KV().Put(&api.KVPair{
        Key:   key,
        Value: []byte(configContent),
    }, nil)
    return err
}

// 添加新集群
func (s *AlertService) AddCluster(name string) error {
    key := fmt.Sprintf("alert/cluster/%s/config", name)
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
func (s *AlertService) DeleteCluster(name string) error {
    prefix := fmt.Sprintf("alert/cluster/%s", name)
    _, err := s.consul.Client.KV().DeleteTree(prefix, nil)
    if err != nil {
        return fmt.Errorf("failed to delete cluster: %v", err)
    }
    return nil
}

// 辅助函数
func getAlertClusterName(key string) string {
    parts := strings.Split(key, "/")
    if len(parts) >= 3 && parts[0] == "alert" && parts[1] == "cluster" {
        return parts[2]
    }
    return ""
} 