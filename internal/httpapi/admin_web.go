package httpapi

import (
	"embed"
	"html/template"
)

const adminWebSessionCookieName = "proofline_admin_session"

//go:embed web/templates/admin.html web/admin/static/*
var adminWebFS embed.FS

var adminWebTemplate = template.Must(template.New("admin.html").Funcs(template.FuncMap{
	"humanTime": humanTime,
}).ParseFS(adminWebFS, "web/templates/admin.html"))
