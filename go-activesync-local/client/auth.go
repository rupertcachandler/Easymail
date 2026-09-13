package client

import (
	"encoding/base64"
	"net/http"
)

// Authenticator decorates outbound HTTP requests with credentials. It is
// applied for every command issued by the high level client. Implementations
// are expected to be safe for concurrent use.
type Authenticator interface {
	Apply(req *http.Request) error
}

// BasicAuth is the canonical RFC 7617 implementation: it sets the
// Authorization header to "Basic " + base64(username:password).
type BasicAuth struct {
	Username string
	Password string
}

// Apply writes the Authorization header onto req.
func (a BasicAuth) Apply(req *http.Request) error {
	cred := a.Username + ":" + a.Password
	enc := base64.StdEncoding.EncodeToString([]byte(cred))
	req.Header.Set("Authorization", "Basic "+enc)
	return nil
}
