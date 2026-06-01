package httpapi

import "net/http"

func (a *API) mainRoutes() http.Handler {
	mux := http.NewServeMux()
	a.registerMainAuthRoutes(mux)
	a.registerAdminAPIRoutes(mux)
	a.registerMainContactRoutes(mux)
	a.registerMainIncidentRoutes(mux)
	a.registerMainStreamRoutes(mux)
	a.registerMainIncidentTokenRoutes(mux)
	a.registerMainSharingGrantRoutes(mux)
	a.registerMainWrappedKeyRoutes(mux)
	a.registerPublicIncidentViewerRoutes(mux)
	mux.HandleFunc("/", a.notFound)

	return a.loggingMiddleware(a.recoveryMiddleware(a.mainSecurityMiddleware(a.publicRateLimitMiddleware(a.mainAPIRouteRateLimitMiddleware(mux)))))
}

func (a *API) registerMainContactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/contact-public-keys", a.withPrivateAuth(a.createContactPublicKey))
	mux.HandleFunc("GET /v1/contact-public-keys", a.withPrivateAuth(a.listContactPublicKeys))
	mux.HandleFunc("GET /v1/contact-public-keys/{public_key_id}", a.withPrivateAuth(a.getContactPublicKey))
	mux.HandleFunc("PATCH /v1/contact-public-keys/{public_key_id}", a.withPrivateAuth(a.updateContactPublicKey))
	mux.HandleFunc("POST /v1/contact-public-keys/{public_key_id}/revoke", a.withPrivateAuth(a.revokeContactPublicKey))
}

func (a *API) adminRoutes() http.Handler {
	mux := http.NewServeMux()
	a.registerPrivateAdminWebRoutes(mux)
	mux.HandleFunc("/", a.notFound)

	return a.loggingMiddleware(a.recoveryMiddleware(a.privateSecurityMiddleware(mux)))
}

func (a *API) registerMainAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/login", a.login)
	mux.HandleFunc("POST /v1/auth/logout", a.withPrivateAuth(a.logout))
	mux.HandleFunc("GET /v1/account", a.withPrivateAuth(a.getCurrentAccount))
	mux.HandleFunc("POST /v1/account/password", a.withPrivateAuth(a.changeOwnPassword))
}

func (a *API) registerAdminAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/accounts", a.withPrivateAuth(a.listAccounts))
	mux.HandleFunc("POST /v1/admin/accounts", a.withPrivateAuth(a.createAccount))
	mux.HandleFunc("POST /v1/admin/accounts/{account_id}/password", a.withPrivateAuth(a.resetAccountPassword))
	mux.HandleFunc("POST /v1/admin/accounts/{account_id}/sessions/revoke", a.withPrivateAuth(a.revokeAccountSessions))
	mux.HandleFunc("GET /v1/admin/incidents/{incident_id}/deletion", a.withPrivateAuth(a.getAdminIncidentDeletion))
	mux.HandleFunc("POST /v1/admin/incidents/{incident_id}/deletion", a.withPrivateAuth(a.requestAdminIncidentDeletion))
}

func (a *API) registerPrivateAdminWebRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", a.adminWebPage)
	mux.HandleFunc("POST /admin/bootstrap", a.adminWebBootstrap)
	mux.HandleFunc("POST /admin/login", a.adminWebLogin)
	mux.HandleFunc("POST /admin/logout", a.adminWebLogout)
	mux.HandleFunc("POST /admin/password", a.adminWebChangeOwnPassword)
	mux.HandleFunc("POST /admin/accounts/{account_id}/password", a.adminWebResetAccountPassword)
	mux.Handle("GET /admin/static/", a.adminWebStaticHandler())
}

func (a *API) registerMainIncidentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/incidents", a.withPrivateAuth(a.createIncident))
	mux.HandleFunc("GET /v1/incidents/{incident_id}", a.withPrivateAuth(a.getIncident))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/deletion", a.withPrivateAuth(a.getIncidentDeletion))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/deletion", a.withPrivateAuth(a.requestIncidentDeletion))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/sharing-grants", a.withPrivateAuth(a.createSharingGrant))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/sharing-grants", a.withPrivateAuth(a.listSharingGrants))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/wrapped-keys", a.withPrivateAuth(a.createWrappedKeyRecord))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/wrapped-keys", a.withPrivateAuth(a.listWrappedKeyRecords))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/chunks/reconcile", a.withPrivateAuth(a.reconcileChunk))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/chunks", a.withPrivateAuth(a.uploadChunk))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/chunks", a.withPrivateAuth(a.listChunks))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/chunks/{media_type}/{chunk_index}", a.withPrivateAuth(a.getChunkBytes))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/download", a.withPrivateAuth(a.downloadPrivateIncidentBundle))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/checkins", a.withPrivateAuth(a.createCheckin))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/close", a.withPrivateAuth(a.closeIncident))
}

func (a *API) registerMainStreamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/incidents/{incident_id}/streams", a.withPrivateAuth(a.createMediaStream))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/streams", a.withPrivateAuth(a.listMediaStreams))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/streams/{stream_id}", a.withPrivateAuth(a.getMediaStream))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/streams/{stream_id}/complete", a.withPrivateAuth(a.completeMediaStream))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/streams/{stream_id}/fail", a.withPrivateAuth(a.failMediaStream))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/streams/{stream_id}/download", a.withPrivateAuth(a.downloadPrivateStreamBundle))
}

func (a *API) registerMainIncidentTokenRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/incidents/{incident_id}/incident-tokens", a.withPrivateAuth(a.createIncidentToken))
	mux.HandleFunc("POST /v1/incident-tokens/{token_id}/revoke", a.withPrivateAuth(a.revokeIncidentToken))
}

func (a *API) registerMainSharingGrantRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sharing-grants/{grant_id}", a.withPrivateAuth(a.getSharingGrant))
	mux.HandleFunc("POST /v1/sharing-grants/{grant_id}/revoke", a.withPrivateAuth(a.revokeSharingGrant))
}

func (a *API) registerMainWrappedKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/wrapped-keys/{wrapped_key_id}", a.withPrivateAuth(a.getWrappedKeyRecord))
	mux.HandleFunc("POST /v1/wrapped-keys/{wrapped_key_id}/revoke", a.withPrivateAuth(a.revokeWrappedKeyRecord))
}

func (a *API) publicRoutes() http.Handler {
	mux := http.NewServeMux()
	a.registerPublicIncidentViewerRoutes(mux)
	mux.HandleFunc("/", a.notFound)

	return a.loggingMiddleware(a.recoveryMiddleware(a.publicSecurityMiddleware(a.publicRateLimitMiddleware(mux))))
}

func (a *API) registerPublicIncidentViewerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /i/{token}", a.incidentViewerPage)
	mux.HandleFunc("GET /i/{token}/data", a.incidentViewData)
	mux.HandleFunc("GET /i/{token}/streams/{stream_id}/download", a.downloadIncidentViewerStreamBundle)
	mux.HandleFunc("GET /i/{token}/incident/download", a.downloadIncidentViewerIncidentBundle)
	// Keep the pre-rename viewer path as a compatibility alias for already
	// shared token-bearing links. /i remains canonical for new links.
	mux.HandleFunc("GET /e/{token}", a.incidentViewerPage)
	mux.HandleFunc("GET /e/{token}/data", a.incidentViewData)
	mux.HandleFunc("GET /e/{token}/streams/{stream_id}/download", a.downloadIncidentViewerStreamBundle)
	mux.HandleFunc("GET /e/{token}/incident/download", a.downloadIncidentViewerIncidentBundle)
	// Static incident viewer assets are embedded and token-neutral; the token stays
	// in the request path handled above.
	mux.Handle("GET /static/", incidentViewerStaticHandler())
}

func (a *API) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "endpoint was not found")
}
