package httpapi

import "net/http"

func (a *API) mainRoutes() http.Handler {
	mux := http.NewServeMux()
	a.registerMainAuthRoutes(mux)
	a.registerMainContactRoutes(mux)
	a.registerMainAccountRecipientKeyRoutes(mux)
	a.registerMainTrustedContactRelationshipRoutes(mux)
	a.registerMainTrustedContactDeliveryRoutes(mux)
	a.registerMainIncidentRoutes(mux)
	a.registerMainStreamRoutes(mux)
	a.registerMainIncidentTokenRoutes(mux)
	a.registerMainSharingGrantRoutes(mux)
	a.registerMainWrappedKeyRoutes(mux)
	a.registerPublicIncidentViewerRoutes(mux)
	mux.HandleFunc("/", a.notFound)

	return a.loggingMiddleware(a.recoveryMiddleware(a.mainSecurityMiddleware(a.webCORSMiddleware(a.publicRateLimitMiddleware(a.mainAPIRouteRateLimitMiddleware(mux))))))
}

func (a *API) registerMainContactRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/contact-public-keys", a.withPrivateAuth(a.createContactPublicKey))
	mux.HandleFunc("GET /v1/contact-public-keys", a.withPrivateAuth(a.listContactPublicKeys))
	mux.HandleFunc("GET /v1/contact-public-keys/{public_key_id}", a.withPrivateAuth(a.getContactPublicKey))
	mux.HandleFunc("PATCH /v1/contact-public-keys/{public_key_id}", a.withPrivateAuth(a.updateContactPublicKey))
	mux.HandleFunc("POST /v1/contact-public-keys/{public_key_id}/revoke", a.withPrivateAuth(a.revokeContactPublicKey))
	mux.HandleFunc("POST /v1/contact-public-keys/{public_key_id}/lost", a.withPrivateAuth(a.markContactPublicKeyLost))
	mux.HandleFunc("POST /v1/contact-public-keys/{public_key_id}/replace", a.withPrivateAuth(a.replaceContactPublicKey))
}

func (a *API) registerMainAccountRecipientKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/account-recipient-keys", a.withPrivateAuth(a.createAccountRecipientKey))
	mux.HandleFunc("GET /v1/account-recipient-keys", a.withPrivateAuth(a.listAccountRecipientKeys))
	mux.HandleFunc("GET /v1/account-recipient-keys/{recipient_key_id}", a.withPrivateAuth(a.getAccountRecipientKey))
	mux.HandleFunc("PATCH /v1/account-recipient-keys/{recipient_key_id}", a.withPrivateAuth(a.updateAccountRecipientKey))
	mux.HandleFunc("POST /v1/account-recipient-keys/{recipient_key_id}/revoke", a.withPrivateAuth(a.revokeAccountRecipientKey))
	mux.HandleFunc("POST /v1/account-recipient-keys/{recipient_key_id}/lost", a.withPrivateAuth(a.markAccountRecipientKeyLost))
	mux.HandleFunc("POST /v1/account-recipient-keys/{recipient_key_id}/replace", a.withPrivateAuth(a.replaceAccountRecipientKey))
}

func (a *API) registerMainTrustedContactRelationshipRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/trusted-contact-relationships", a.withPrivateAuth(a.createTrustedContactRelationship))
	mux.HandleFunc("GET /v1/trusted-contact-relationships", a.withPrivateAuth(a.listTrustedContactRelationships))
	mux.HandleFunc("GET /v1/trusted-contact-relationships/{relationship_id}", a.withPrivateAuth(a.getTrustedContactRelationship))
	mux.HandleFunc("POST /v1/trusted-contact-relationships/{relationship_id}/accept", a.withPrivateAuth(a.acceptTrustedContactRelationship))
	mux.HandleFunc("POST /v1/trusted-contact-relationships/{relationship_id}/decline", a.withPrivateAuth(a.declineTrustedContactRelationship))
	mux.HandleFunc("POST /v1/trusted-contact-relationships/{relationship_id}/revoke", a.withPrivateAuth(a.revokeTrustedContactRelationship))
	mux.HandleFunc("POST /v1/trusted-contact-relationships/{relationship_id}/replace", a.withPrivateAuth(a.replaceTrustedContactRelationship))
}

func (a *API) registerMainTrustedContactDeliveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/trusted-contact/incidents/{incident_id}/wrapped-keys", a.withPrivateAuth(a.listTrustedContactWrappedKeyRecords))
	mux.HandleFunc("GET /v1/trusted-contact/wrapped-keys/{wrapped_key_id}", a.withPrivateAuth(a.getTrustedContactWrappedKeyRecord))
}

func (a *API) adminRoutes() http.Handler {
	mux := http.NewServeMux()
	a.registerAdminAPIRoutes(mux)
	a.registerPrivateAdminWebRoutes(mux)
	mux.HandleFunc("/", a.notFound)

	return a.loggingMiddleware(a.recoveryMiddleware(a.privateSecurityMiddleware(mux)))
}

func (a *API) registerMainAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/login", a.login)
	mux.HandleFunc("POST /v1/auth/register", a.registerAccount)
	mux.HandleFunc("POST /v1/auth/email/verify", a.verifyAccountEmail)
	mux.HandleFunc("POST /v1/auth/logout", a.withPrivateAuth(a.logout))
	mux.HandleFunc("POST /v1/auth/web/login", a.webLogin)
	mux.HandleFunc("POST /v1/auth/web/logout", a.webLogout)
	mux.HandleFunc("GET /v1/auth/web/csrf", a.withPrivateAuth(a.webCSRF))
	mux.HandleFunc("GET /v1/account", a.withPrivateAuth(a.getCurrentAccount))
	mux.HandleFunc("POST /v1/account/password", a.withPrivateAuth(a.changeOwnPassword))
	mux.HandleFunc("POST /v1/account/second-factor/email/challenge", a.withPrivateAuth(a.requestEmailSecondFactorChallenge))
	mux.HandleFunc("POST /v1/account/second-factor/email/verify", a.withPrivateAuth(a.verifyEmailSecondFactorChallenge))
	mux.HandleFunc("POST /v1/account/second-factor/totp/enroll", a.withPrivateAuth(a.startTOTPSecondFactorEnrollment))
	mux.HandleFunc("POST /v1/account/second-factor/totp/confirm", a.withPrivateAuth(a.confirmTOTPSecondFactorEnrollment))
	mux.HandleFunc("POST /v1/account/second-factor/totp/verify", a.withPrivateAuth(a.verifyTOTPSecondFactorChallenge))
	mux.HandleFunc("POST /v1/account/second-factor/webauthn/register/start", a.withPrivateAuth(a.startWebAuthnRegistration))
	mux.HandleFunc("POST /v1/account/second-factor/webauthn/register/finish", a.withPrivateAuth(a.finishWebAuthnRegistration))
	mux.HandleFunc("POST /v1/account/second-factor/webauthn/verify/start", a.withPrivateAuth(a.startWebAuthnVerification))
	mux.HandleFunc("POST /v1/account/second-factor/webauthn/verify/finish", a.withPrivateAuth(a.finishWebAuthnVerification))
}

func (a *API) registerAdminAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/accounts", a.withPrivateAuth(a.listAccounts))
	mux.HandleFunc("POST /v1/admin/accounts", a.withPrivateAuth(a.createAccount))
	mux.HandleFunc("POST /v1/admin/accounts/{account_id}/password", a.withPrivateAuth(a.resetAccountPassword))
	mux.HandleFunc("POST /v1/admin/accounts/{account_id}/second-factor/recovery/reset", a.withPrivateAuth(a.resetAccountSecondFactorRecovery))
	mux.HandleFunc("POST /v1/admin/accounts/{account_id}/sessions/revoke", a.withPrivateAuth(a.revokeAccountSessions))
	mux.HandleFunc("GET /v1/admin/incidents/unowned", a.withPrivateAuth(a.listLegacyUnownedIncidentCandidates))
	mux.HandleFunc("GET /v1/admin/incidents/{incident_id}/deletion", a.withPrivateAuth(a.getAdminIncidentDeletion))
	mux.HandleFunc("POST /v1/admin/incidents/{incident_id}/deletion", a.withPrivateAuth(a.requestAdminIncidentDeletion))
	mux.HandleFunc("POST /v1/admin/incidents/{incident_id}/reassignment", a.withPrivateAuth(a.reassignLegacyUnownedIncident))
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
	mux.HandleFunc("GET /v1/incidents", a.withPrivateAuth(a.listAccountIncidents))
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
	mux.HandleFunc("GET /v1/incidents/{incident_id}/incident-tokens", a.withPrivateAuth(a.listIncidentTokens))
	mux.HandleFunc("POST /v1/incidents/{incident_id}/incident-tokens", a.withPrivateAuth(a.createIncidentToken))
	mux.HandleFunc("GET /v1/incidents/{incident_id}/incident-tokens/{token_id}", a.withPrivateAuth(a.getIncidentTokenMetadata))
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
	mux.HandleFunc("GET /i/{token}/viewer-payload", a.webClientViewerPayload)
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
