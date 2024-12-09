package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"github.com/gorilla/mux"
	"consul-ui/types"
)

// 处理Prometheus配置页面
func (app *Application) HandlePromConfigs(w http.ResponseWriter, r *http.Request) {
	clusters, err := app.PromService.ListClusters()
	if err != nil {
		app.Logger.Printf("Error listing clusters: %v", err)
		http.Error(w, "Failed to list clusters", http.StatusInternalServerError)
		return
	}

	data := struct {
		Clusters []types.Cluster
		Content  string
	}{
		Clusters: clusters,
		Content:  "prometheus-configs",
	}
	
	if err := app.Templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		app.Logger.Printf("Template execution error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// API处理函数

// 获取配置
func (app *Application) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	config, err := app.PromService.GetConfig(clusterName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get config: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(config))
}

// 保存配置
func (app *Application) HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	config, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	if err := app.PromService.SaveConfig(clusterName, string(config)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 添加新集群
func (app *Application) HandleAddCluster(w http.ResponseWriter, r *http.Request) {
	var cluster types.Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := app.PromService.AddCluster(cluster.Name); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add cluster: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
}

// 删除集群
func (app *Application) HandleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	if err := app.PromService.DeleteCluster(clusterName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete cluster: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 处理告警规则页面 - 显示集群列表
func (app *Application) HandlePromRules(w http.ResponseWriter, r *http.Request) {
	clusters, err := app.PromService.ListClusters()
	if err != nil {
		app.Logger.Printf("Error listing clusters: %v", err)
		http.Error(w, "Failed to list clusters", http.StatusInternalServerError)
		return
	}

	data := struct {
		Clusters []types.Cluster
		Content  string
	}{
		Clusters: clusters,
		Content:  "prometheus-rules",
	}
	
	if err := app.Templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		app.Logger.Printf("Template execution error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// 处理特定集群的告警规则
func (app *Application) HandleClusterRules(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	rules, err := app.PromService.ListRules(clusterName)
	if err != nil {
		app.Logger.Printf("Error listing rules: %v", err)
		http.Error(w, "Failed to list rules", http.StatusInternalServerError)
		return
	}
	
	
	data := struct {
		Rules    []types.Rule
		Content  string
		Cluster  string
	}{
		Rules:    rules,
		Content:  "prometheus-cluster-rules",
		Cluster:  clusterName,
	}
	
	if err := app.Templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		app.Logger.Printf("Template execution error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// API处理函数

// 获取规则
func (app *Application) HandleGetRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	ruleFile := vars["rule"]
	
	rule, err := app.PromService.GetRule(clusterName, ruleFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get rule: %v", err), http.StatusInternalServerError)
		return
	}
	
	if rule == nil {
		http.NotFound(w, r)
		return
	}
	
	json.NewEncoder(w).Encode(rule)
}

// 保存规则
func (app *Application) HandleSaveRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	ruleFile := vars["rule"]
	
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	if err := app.PromService.SaveRule(clusterName, ruleFile, string(content)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save rule: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 删除规则
func (app *Application) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	ruleFile := vars["rule"]
	
	if err := app.PromService.DeleteRule(clusterName, ruleFile); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete rule: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 切换规则状态
func (app *Application) HandleToggleRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	ruleFile := vars["rule"]
	
	var req struct {
		Enable bool `json:"enable"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := app.PromService.ToggleRule(clusterName, ruleFile, req.Enable); err != nil {
		http.Error(w, fmt.Sprintf("Failed to toggle rule: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}




