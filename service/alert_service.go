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

// 获取模板列表
func (s *AlertService) ListTemplates(clusterName string) ([]types.Rule, error) {
    prefix := fmt.Sprintf("alert/cluster/%s/tmpl/", clusterName)
    pairs, _, err := s.consul.Client.KV().List(prefix, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to list templates: %v", err)
    }

    tmplMap := make(map[string]types.Rule)
    for _, pair := range pairs {
        parts := strings.Split(pair.Key, "/")
        if len(parts) < 5 {
            continue
        }
        
        tmplFile := parts[4]
        if strings.HasSuffix(pair.Key, "/tmpl") {
            rule, exists := tmplMap[tmplFile]
            if !exists {
                rule = types.Rule{RuleFile: tmplFile}
            }
            rule.Content = string(pair.Value)
            tmplMap[tmplFile] = rule
        } else if strings.HasSuffix(pair.Key, "/enable") {
            rule, exists := tmplMap[tmplFile]
            if !exists {
                rule = types.Rule{RuleFile: tmplFile}
            }
            rule.Enable = string(pair.Value) == "true"
            tmplMap[tmplFile] = rule
        }
    }

    templates := make([]types.Rule, 0)
    for _, rule := range tmplMap {
        templates = append(templates, rule)
    }
    return templates, nil
}

// 获取模板
func (s *AlertService) GetTemplate(clusterName, tmplFile string) (string, error) {
    key := fmt.Sprintf("alert/cluster/%s/tmpl/%s/tmpl", clusterName, tmplFile)
    pair, _, err := s.consul.Client.KV().Get(key, nil)
    if err != nil {
        return "", fmt.Errorf("failed to get template: %v", err)
    }
    if pair == nil {
        return "", nil
    }
    return string(pair.Value), nil
}

// 保存模板
func (s *AlertService) SaveTemplate(clusterName, tmplFile, content string) error {
    // TODO: 添加模板语法验证
    contentKey := fmt.Sprintf("alert/cluster/%s/tmpl/%s/tmpl", clusterName, tmplFile)
    enableKey := fmt.Sprintf("alert/cluster/%s/tmpl/%s/enable", clusterName, tmplFile)
    
    // 保存模板内容
    _, err := s.consul.Client.KV().Put(&api.KVPair{
        Key:   contentKey,
        Value: []byte(content),
    }, nil)
    if err != nil {
        return fmt.Errorf("failed to save template content: %v", err)
    }
    
    // 默认启用模板
    _, err = s.consul.Client.KV().Put(&api.KVPair{
        Key:   enableKey,
        Value: []byte("true"),
    }, nil)
    if err != nil {
        return fmt.Errorf("failed to save template status: %v", err)
    }
    
    return nil
}

// 删除模板
func (s *AlertService) DeleteTemplate(clusterName, tmplFile string) error {
    prefix := fmt.Sprintf("alert/cluster/%s/tmpl/%s", clusterName, tmplFile)
    _, err := s.consul.Client.KV().DeleteTree(prefix, nil)
    if err != nil {
        return fmt.Errorf("failed to delete template: %v", err)
    }
    return nil
}

// 切换模板状态
func (s *AlertService) ToggleTemplate(clusterName, tmplFile string, enable bool) error {
    key := fmt.Sprintf("alert/cluster/%s/tmpl/%s/enable", clusterName, tmplFile)
    _, err := s.consul.Client.KV().Put(&api.KVPair{
        Key:   key,
        Value: []byte(fmt.Sprintf("%v", enable)),
    }, nil)
    if err != nil {
        return fmt.Errorf("failed to toggle template: %v", err)
    }
    return nil
}