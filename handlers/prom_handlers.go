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



func (app *Application) HandlePromRules(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Content string
	}{
		Content: "prometheus-rules",
	}
	app.Templates.ExecuteTemplate(w, "layout.html", data)
}




