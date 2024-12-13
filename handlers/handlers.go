package handlers

import (
    "net/http"
    "html/template"
    "consul-ui/service"
    "log"
    "os"
)

type Application struct {
    Templates *template.Template
    PromService *service.PromService
    AlertService *service.AlertService
    Logger *log.Logger
}

func NewApplication(templates *template.Template, address string, token string) *Application {
    consulService, err := service.NewConsulService(address, token)
    if err != nil {
        panic(err)
    }
    promService := service.NewPromService(consulService)
    alertService := service.NewAlertService(consulService)
    return &Application{
        Templates: templates,
        PromService: promService,
        AlertService: alertService,
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

