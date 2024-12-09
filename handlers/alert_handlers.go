package handlers

import "net/http"


func (app *Application) HandleAlertConfigs(w http.ResponseWriter, r *http.Request) {
    // 类似的实现...

    data := struct {
        Content string
    }{
        Content: "alertmanager-configs",
    }
    app.Templates.ExecuteTemplate(w, "layout.html", data)
}

func (app *Application) HandleAlertRules(w http.ResponseWriter, r *http.Request) {
    // 类似的实现...
    data := struct {
        Content string
    }{
        Content: "alertmanager-rules",
    }
    app.Templates.ExecuteTemplate(w, "layout.html", data)
}