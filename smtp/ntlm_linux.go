package smtp

import "errors"

// PlainAuth returns an Auth that implements the PLAIN authentication
// mechanism as defined in RFC 4616.
// The returned Auth uses the given username and password to authenticate
// on TLS connections to host and act as identity. Usually identity will be
// left blank to act as username.
func NTLMAuth(host, user, password, workstation string) *ntlmAuth {
	return &ntlmAuth{initErr: errors.New("ntlm not support")}
}

// NTLMAuth implements smtp.Auth. The authentication mechanism.
type ntlmAuth struct {
	initErr error
}

func (n *ntlmAuth) Start(server *ServerInfo) (string, []byte, error) {
	return "", nil, n.initErr
}

func (n *ntlmAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	return nil, n.initErr
}
