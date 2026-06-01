package httpapi_test

import (
	"net/http"
	"testing"
)

func TestV1HealthRoutesAreNotMountedOnMainOrAdminHandlers(t *testing.T) {
	app := newTestApp(t)

	for _, handler := range []struct {
		name    string
		handler http.Handler
	}{
		{name: "main", handler: app.mainHandler},
		{name: "admin", handler: app.adminHandler},
	} {
		for _, target := range []string{"/v1/health/live", "/v1/health/ready"} {
			response, body := request(t, handler.handler, http.MethodGet, target, "", nil)
			response.Body.Close()
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("%s handler %s: expected status 404, got %d: %s", handler.name, target, response.StatusCode, body)
			}
			assertErrorCode(t, body, "not_found")
		}
	}
}
