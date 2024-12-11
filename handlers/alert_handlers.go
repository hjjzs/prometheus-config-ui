package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"github.com/gorilla/mux"
	"regexp"
    "consul-ui/types"
)

// 处理Alertmanager配置页面
func (app *Application) HandleAlertConfigs(w http.ResponseWriter, r *http.Request) {
	clusters, err := app.AlertService.ListClusters()
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
		Content:  "alertmanager-configs",
	}
	
	if err := app.Templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		app.Logger.Printf("Template execution error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// API处理函数

// 获取配置
func (app *Application) HandleGetAlertConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	config, err := app.AlertService.GetConfig(clusterName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get config: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(config))
}

// 保存配置
func (app *Application) HandleSaveAlertConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	config, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	if err := app.AlertService.SaveConfig(clusterName, string(config)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 添加新集群
func (app *Application) HandleAddAlertCluster(w http.ResponseWriter, r *http.Request) {
	var cluster types.Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证集群名称
	if len(cluster.Name) == 0 {
		http.Error(w, "集群名称不能为空", http.StatusBadRequest)
		return
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(cluster.Name) {
		http.Error(w, "集群名称只允许字母、数字、下划线、点、中划线", http.StatusBadRequest)
		return
	}

	if len(cluster.Name) > 30 {
		http.Error(w, "集群名称超过最大长度限制(30)", http.StatusBadRequest)
		return
	}

	if err := app.AlertService.AddCluster(cluster.Name); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add cluster: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
}

// 删除集群
func (app *Application) HandleDeleteAlertCluster(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	
	if err := app.AlertService.DeleteCluster(clusterName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete cluster: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

// 获取告警模板
func (app *Application) HandleAlertTmpl(w http.ResponseWriter, r *http.Request) {
	
    data := struct {
     
        Content  string
    }{
      
        Content:  "alertmanager-tmpls",
    }
    
    app.Templates.ExecuteTemplate(w, "layout.html", data)
	
}
