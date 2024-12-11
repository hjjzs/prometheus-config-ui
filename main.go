package main

import (
    "html/template"
    "net/http"
    "github.com/gorilla/mux"
	"consul-ui/handlers"
	"os"
	"time"
	"context"
	"os/signal"
	"syscall"
	"fmt"
)

func main() {
    
    
    // 初始化Consul客户端
    // config := api.DefaultConfig()
    // config.Address = "192.168.48.129:8500"
    // config.Token = "5e7f0c19-73ac-6023-c8ba-eb77988cd641"
    // client, err := api.NewClient(config)
    // if err != nil {
    //     panic(err)
    // }
	
    // 初始化模板
    templates := template.Must(template.ParseGlob("templates/*"))
    
	app := handlers.NewApplication(templates)
    // 设置路由
    r := mux.NewRouter()
    
    // 静态文件服务
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    
    // 页面路由
    r.HandleFunc("/", app.HandleHome)
    r.HandleFunc("/prometheus/configs", app.HandlePromConfigs)
    r.HandleFunc("/prometheus/rules", app.HandlePromRules)
    r.HandleFunc("/prometheus/rules/{cluster}", app.HandleClusterRules)
    r.HandleFunc("/alertmanager/configs", app.HandleAlertConfigs)
    r.HandleFunc("/alertmanager/tmpls", app.HandleAlertTmpl)
    r.HandleFunc("/alertmanager/tmpls/{cluster}", app.HandleClusterTemplates)
    // r.HandleFunc("/users", app.HandleUsers)
    // r.HandleFunc("/roles", app.HandleRoles)

    // 添加新的API路由
    r.HandleFunc("/api/prometheus/configs/{cluster}", app.HandleGetConfig).Methods("GET")
    r.HandleFunc("/api/prometheus/configs/{cluster}", app.HandleSaveConfig).Methods("POST")
    r.HandleFunc("/api/prometheus/clusters", app.HandleAddCluster).Methods("POST")
    r.HandleFunc("/api/prometheus/clusters/{cluster}", app.HandleDeleteCluster).Methods("DELETE")
    r.HandleFunc("/prometheus/rules/{cluster}", app.HandlePromRules)
    r.HandleFunc("/api/prometheus/rules/{cluster}/{rule}", app.HandleGetRule).Methods("GET")
    r.HandleFunc("/api/prometheus/rules/{cluster}/{rule}", app.HandleSaveRule).Methods("POST")
    r.HandleFunc("/api/prometheus/rules/{cluster}/{rule}", app.HandleDeleteRule).Methods("DELETE")
    r.HandleFunc("/api/prometheus/rules/{cluster}/{rule}/toggle", app.HandleToggleRule).Methods("POST")

    // Alertmanager API路由
    r.HandleFunc("/api/alertmanager/configs/{cluster}", app.HandleGetAlertConfig).Methods("GET")
    r.HandleFunc("/api/alertmanager/configs/{cluster}", app.HandleSaveAlertConfig).Methods("POST")
    r.HandleFunc("/api/alertmanager/clusters", app.HandleAddAlertCluster).Methods("POST")
    r.HandleFunc("/api/alertmanager/clusters/{cluster}", app.HandleDeleteAlertCluster).Methods("DELETE")
    r.HandleFunc("/api/alertmanager/tmpl/{cluster}/{tmpl}", app.HandleGetAlertTmpl).Methods("GET")
    r.HandleFunc("/api/alertmanager/tmpl/{cluster}/{tmpl}", app.HandleSaveAlertTmpl).Methods("POST")
    r.HandleFunc("/api/alertmanager/tmpl/{cluster}/{tmpl}", app.HandleDeleteAlertTmpl).Methods("DELETE")
    r.HandleFunc("/api/alertmanager/tmpl/{cluster}/{tmpl}/toggle", app.HandleToggleAlertTmpl).Methods("POST")

    // 优雅退出程序
    srv := &http.Server{
        Addr:    ":8080",
        Handler: r,
    }

    // 在单独的goroutine中启动服务器
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            panic(err)
        }
    }()

	fmt.Println("服务器启动")

    // 等待中断信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// fmt.Println("等待中断信号")
    <-quit

    // 收到中断信号后优雅关闭服务器
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    fmt.Println("关闭服务器")
    if err := srv.Shutdown(ctx); err != nil {
        panic(err)
    }
} 