package httpapi

import (
	"context"

	"github.com/open-proofline/server/internal/auth"
)

type privateAuthSource string

const (
	privateAuthSourceBearer    privateAuthSource = "bearer"
	privateAuthSourceWebCookie privateAuthSource = "web_cookie"
)

type privatePrincipal struct {
	Account    auth.Account
	Session    auth.Session
	AuthSource privateAuthSource
}

type privatePrincipalContextKey struct{}

func contextWithPrincipal(ctx context.Context, principal privatePrincipal) context.Context {
	return context.WithValue(ctx, privatePrincipalContextKey{}, principal)
}

func principalFromContext(ctx context.Context) (privatePrincipal, bool) {
	principal, ok := ctx.Value(privatePrincipalContextKey{}).(privatePrincipal)
	return principal, ok
}
