package httpapi

import (
	"io/fs"
	"net/http"
)

func (a *API) adminWebStaticHandler() http.Handler {
	staticFiles, err := fs.Sub(adminWebFS, "web/admin/static")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFiles)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAdminWebStaticHeaders(w)
		fileServer.ServeHTTP(w, r)
	})
}
