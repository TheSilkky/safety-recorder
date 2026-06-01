package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

type accountResponse struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Role              string    `json:"role"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	PasswordChangedAt time.Time `json:"password_changed_at"`
}

type authSessionResponse struct {
	SessionID string          `json:"session_id"`
	Account   accountResponse `json:"account"`
	Token     string          `json:"token"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	account, err := a.repo.GetAccountByUsername(r.Context(), auth.NormalizeUsername(request.Username))
	if errors.Is(err, auth.ErrNotFound) {
		auth.SpendPasswordHashCost(request.Password, a.passwordCost)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is invalid")
		return
	}
	if err != nil {
		a.internalError(w, "get login account", err)
		return
	}
	if !auth.VerifyPassword(account.PasswordHash, request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is invalid")
		return
	}

	session, rawToken, err := a.repo.CreateSession(r.Context(), account.ID, time.Now().UTC().Add(a.sessionTTL))
	if err != nil {
		a.internalError(w, "create auth session", err)
		return
	}
	writeJSON(w, http.StatusCreated, authSessionResponse{
		SessionID: session.ID,
		Account:   makeAccountResponse(account),
		Token:     rawToken,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	if err := a.repo.RevokeSession(r.Context(), principal.Session.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
		a.internalError(w, "revoke auth session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *API) getCurrentAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]accountResponse{
		"account": makeAccountResponse(principal.Account),
	})
}

func (a *API) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if !auth.VerifyPassword(principal.Account.PasswordHash, request.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "current password is invalid")
		return
	}
	account, ok := a.updateAccountPassword(w, r, principal.Account.ID, request.NewPassword)
	if !ok {
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), principal.Account.ID, principal.Session.ID); err != nil {
		a.internalError(w, "revoke other account sessions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]accountResponse{
		"account": makeAccountResponse(account),
	})
}

func (a *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	accounts, err := a.repo.ListAccounts(r.Context())
	if err != nil {
		a.internalError(w, "list accounts", err)
		return
	}
	response := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, makeAccountResponse(account))
	}
	writeJSON(w, http.StatusOK, map[string][]accountResponse{"accounts": response})
}

func (a *API) createAccount(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	account, ok := a.createAccountFromRequest(w, r, request.Username, request.Password, request.Role)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]accountResponse{
		"account": makeAccountResponse(account),
	})
}

func (a *API) resetAccountPassword(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	var request struct {
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	account, ok := a.updateAccountPassword(w, r, r.PathValue("account_id"), request.NewPassword)
	if !ok {
		return
	}
	revoked, err := a.repo.RevokeAccountSessions(r.Context(), account.ID, "")
	if err != nil {
		a.internalError(w, "revoke account sessions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account":          makeAccountResponse(account),
		"sessions_revoked": revoked,
	})
}

func (a *API) revokeAccountSessions(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	accountID := r.PathValue("account_id")
	if _, err := a.repo.GetAccountByID(r.Context(), accountID); errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return
	} else if err != nil {
		a.internalError(w, "get account", err)
		return
	}
	revoked, err := a.repo.RevokeAccountSessions(r.Context(), accountID, "")
	if err != nil {
		a.internalError(w, "revoke account sessions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id":       accountID,
		"sessions_revoked": revoked,
	})
}

func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return false
	}
	if principal.Account.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin role is required")
		return false
	}
	return true
}

func (a *API) createAccountFromRequest(w http.ResponseWriter, r *http.Request, username, password, role string) (auth.Account, bool) {
	username = auth.NormalizeUsername(username)
	role = strings.TrimSpace(role)
	if err := auth.ValidateUsername(username); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return auth.Account{}, false
	}
	if !auth.ValidRole(role) {
		writeError(w, http.StatusBadRequest, "invalid_role", "role must be user or admin")
		return auth.Account{}, false
	}
	passwordHash, err := auth.HashPassword(password, a.passwordCost)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return auth.Account{}, false
	}
	account, err := a.repo.CreateAccount(r.Context(), auth.CreateAccountParams{
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		writeError(w, http.StatusConflict, "username_conflict", "username is already in use")
		return auth.Account{}, false
	}
	if err != nil {
		a.internalError(w, "create account", err)
		return auth.Account{}, false
	}
	return account, true
}

func (a *API) updateAccountPassword(w http.ResponseWriter, r *http.Request, accountID, password string) (auth.Account, bool) {
	passwordHash, err := auth.HashPassword(password, a.passwordCost)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return auth.Account{}, false
	}
	account, err := a.repo.UpdateAccountPassword(r.Context(), accountID, passwordHash)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return auth.Account{}, false
	}
	if err != nil {
		a.internalError(w, "update account password", err)
		return auth.Account{}, false
	}
	return account, true
}

func sameSecret(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	wantHash := sha256.Sum256([]byte(want))
	gotHash := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(wantHash[:], gotHash[:]) == 1
}

func makeAccountResponse(account auth.Account) accountResponse {
	return accountResponse{
		ID:                account.ID,
		Username:          account.Username,
		Role:              account.Role,
		CreatedAt:         account.CreatedAt,
		UpdatedAt:         account.UpdatedAt,
		PasswordChangedAt: account.PasswordChangedAt,
	}
}
