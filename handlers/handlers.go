package handlers

import (
    "net/http"
    "html/template"
    "github.com/hashicorp/consul/api"
    "consul-ui/service"
    "log"
    "os"
)

type Application struct {
    Consul    *api.Client
    Templates *template.Template
    ConsulService *service.ConsulService
    Logger *log.Logger
}

func NewApplication(templates *template.Template) *Application {
    address := "192.168.48.129:8500"
    token := "5e7f0c19-73ac-6023-c8ba-eb77988cd641"
    consulService, err := service.NewConsulService(address, token)
    if err != nil {
        panic(err)
    }
    return &Application{
        Consul: consulService.Client,
        Templates: templates,
        ConsulService: consulService,
        Logger: log.New(os.Stdout, "consul-ui: ", log.LstdFlags),
    }
}

func (app *Application) HandleHome(w http.ResponseWriter, r *http.Request) {
    err := app.Templates.ExecuteTemplate(w, "layout.html", nil)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

