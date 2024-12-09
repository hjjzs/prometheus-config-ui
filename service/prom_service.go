package service

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"consul-ui/types"
	"strings"
	promconfig "github.com/prometheus/prometheus/config"
	"gopkg.in/yaml.v3"
	"github.com/prometheus/prometheus/model/rulefmt"
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


// 获取规则列表
func (s *PromService) ListRules(clusterName string) ([]types.Rule, error) {
	prefix := fmt.Sprintf("prom/cluster/%s/rules/", clusterName)
	pairs, _, err := s.consul.Client.KV().List(prefix, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %v", err)
	}

	ruleMap := make(map[string]*types.Rule)
	for _, pair := range pairs {
		parts := strings.Split(pair.Key, "/")
		if len(parts) < 5 {
			continue
		}
		
		ruleFile := parts[4]
		if _, exists := ruleMap[ruleFile]; !exists {
			ruleMap[ruleFile] = &types.Rule{
				RuleFile: ruleFile,
			}
		}

		if strings.HasSuffix(pair.Key, "/rules") {
			ruleMap[ruleFile].Content = string(pair.Value)
		} else if strings.HasSuffix(pair.Key, "/enable") {
			ruleMap[ruleFile].Enable = string(pair.Value) == "true"
		}
	}

	rules := make([]types.Rule, 0)
	for _, rule := range ruleMap {
		if rule.Content != "" {
			rules = append(rules, *rule)
		}
	}
	return rules, nil
}

// 获取单个规则
func (s *PromService) GetRule(clusterName, ruleFile string) (*types.Rule, error) {
	contentKey := fmt.Sprintf("prom/cluster/%s/rules/%s/rules", clusterName, ruleFile)
	enableKey := fmt.Sprintf("prom/cluster/%s/rules/%s/enable", clusterName, ruleFile)
	
	contentPair, _, err := s.consul.Client.KV().Get(contentKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule content: %v", err)
	}
	
	enablePair, _, err := s.consul.Client.KV().Get(enableKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule status: %v", err)
	}
	
	if contentPair == nil {
		return nil, nil
	}
	
	return &types.Rule{
		RuleFile: ruleFile,
		Content:  string(contentPair.Value),
		Enable:   enablePair != nil && string(enablePair.Value) == "true",
	}, nil
}

// 保存规则
func (s *PromService) SaveRule(clusterName, ruleFile, content string) error {
	// 验证规则格式
	if err := validateRuleContent(content); err != nil {
		return fmt.Errorf("invalid rule format: %v", err)
	}
	
	contentKey := fmt.Sprintf("prom/cluster/%s/rules/%s/rules", clusterName, ruleFile)
	enableKey := fmt.Sprintf("prom/cluster/%s/rules/%s/enable", clusterName, ruleFile)
	
	// 保存规则内容
	_, err := s.consul.Client.KV().Put(&api.KVPair{
		Key:   contentKey,
		Value: []byte(content),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to save rule content: %v", err)
	}
	
	// 默认启用规则
	_, err = s.consul.Client.KV().Put(&api.KVPair{
		Key:   enableKey,
		Value: []byte("true"),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to save rule status: %v", err)
	}
	
	return nil
}

// 删除规则
func (s *PromService) DeleteRule(clusterName, ruleFile string) error {
	prefix := fmt.Sprintf("prom/cluster/%s/rules/%s", clusterName, ruleFile)
	_, err := s.consul.Client.KV().DeleteTree(prefix, nil)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %v", err)
	}
	return nil
}

// 切换规则状态
func (s *PromService) ToggleRule(clusterName, ruleFile string, enable bool) error {
	key := fmt.Sprintf("prom/cluster/%s/rules/%s/enable", clusterName, ruleFile)
	_, err := s.consul.Client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(fmt.Sprintf("%v", enable)),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to toggle rule: %v", err)
	}
	return nil
}

// 验证规则内容
func validateRuleContent(content string) error {
	_, errs := rulefmt.Parse([]byte(content))
	if len(errs) > 0 {
		return fmt.Errorf("invalid rule content: %v", errs)
	}
	return nil
}