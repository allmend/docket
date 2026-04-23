package auth

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrProviderUnsupported = errors.New("operation not supported by this provider")
)

// LocalProvider authenticates against bcrypt hashes stored in user_passwords.
// The actual DB lookup is done by the AuthService; this provider just validates
// the form submission and returns the identity for the service layer to act on.
type LocalProvider struct{}

func NewLocalProvider() *LocalProvider {
	return &LocalProvider{}
}

// Authenticate extracts username and password from the POST form.
// Credential validation happens in AuthService.LoginLocal which calls this.
func (p *LocalProvider) Authenticate(_ context.Context, r *http.Request) (*Identity, error) {
	if err := r.ParseForm(); err != nil {
		return nil, ErrInvalidCredentials
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	// Return a partial identity — service layer fills in the rest after DB lookup.
	return &Identity{
		Username: username,
		// Password carried out-of-band; service layer validates hash.
		ExternalID: password, // temporary: service reads this as the raw password
	}, nil
}

// Callback is not used for local auth.
func (p *LocalProvider) Callback(_ context.Context, _ *http.Request) (*Identity, error) {
	return nil, ErrProviderUnsupported
}
