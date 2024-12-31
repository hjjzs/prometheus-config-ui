package handlers

import (
	"net/http"
	"consul-ui/types"
)

// todo 实现用户管理模块

func (app *Application) HandleUsers(w http.ResponseWriter, r *http.Request) {
	users := []types.User{
		{
			Username: "admin",
			Token:    "admin-token-xxx",
			Role:     "管理员",
		},
		{
			Username: "user1",
			Token:    "user1-token-xxx",
			Role:     "普通用户",
		},
	}

	data := struct {
		Users []types.User
		Content string
	}{
		Users: users,
		Content: "users",
	}

	err := app.Templates.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
} 

func (app *Application) HandleRoles(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Content string
	}{
		Content: "roles",
	}
	app.Templates.ExecuteTemplate(w, "layout.html", data)
}

