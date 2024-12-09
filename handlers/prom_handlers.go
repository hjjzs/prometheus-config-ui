package handlers

import (
	"net/http"
)

// 处理Prometheus配置
func (app *Application) HandlePromConfigs(w http.ResponseWriter, r *http.Request) {

	data := struct {
		Content string
	}{
		Content: "prometheus-configs",
	}
	
	if err := app.Templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		app.Logger.Printf("Template execution error: %v", err)
		return
	}
}


// 处理Prometheus规则
func (app *Application) HandlePromRules(w http.ResponseWriter, r *http.Request) {
	_ = app.Consul.KV()

	data := struct {
		Content string
	}{
		Content: "prometheus-rules",
	}
	app.Templates.ExecuteTemplate(w, "layout.html", data)
}





