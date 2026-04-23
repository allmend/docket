package auth

import (
	"context"
	"net/http"
)

// Identity is the normalised result of any authentication provider.
type Identity struct {
	ExternalID string
	Username   string
	Name       string
	Email      string
	Groups     []string
}

// Provider is the interface every identity provider must implement.
// Local and LDAP providers handle credentials in Authenticate.
// OIDC uses Authenticate to return the redirect URL and Callback to finish.
type Provider interface {
	Authenticate(ctx context.Context, r *http.Request) (*Identity, error)
	Callback(ctx context.Context, r *http.Request) (*Identity, error)
}
