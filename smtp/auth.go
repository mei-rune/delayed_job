// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package smtp

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Auth is implemented by an SMTP authentication mechanism.
type Auth interface {
	// Start begins an authentication with a server.
	// It returns the name of the authentication protocol
	// and optionally data to include in the initial AUTH message
	// sent to the server. It can return proto == "" to indicate
	// that the authentication should be skipped.
	// If it returns a non-nil error, the SMTP client aborts
	// the authentication attempt and closes the connection.
	Start(server *ServerInfo) (proto string, toServer []byte, err error)

	// Next continues the authentication. The server has just sent
	// the fromServer data. If more is true, the server expects a
	// response, which Next should return as toServer; otherwise
	// Next should return toServer == nil.
	// If Next returns a non-nil error, the SMTP client aborts
	// the authentication attempt and closes the connection.
	Next(fromServer []byte, more bool) (toServer []byte, err error)
}

// ServerInfo records information about an SMTP server.
type ServerInfo struct {
	Name string   // SMTP server name
	TLS  bool     // using TLS, with valid certificate for Name
	Auth []string // advertised authentication mechanisms
}

type plainAuth struct {
	identity, username, password string
	host                         string

	tryNTLM bool
	auth    Auth
}

// PlainAuth returns an Auth that implements the PLAIN authentication
// mechanism as defined in RFC 4616.
// The returned Auth uses the given username and password to authenticate
// on TLS connections to host and act as identity. Usually identity will be
// left blank to act as username.
func PlainAuth(identity, username, password, host string, tryNTLM bool) Auth {
	return &plainAuth{identity, username, password, host, tryNTLM, nil}
}

func (a *plainAuth) Start(server *ServerInfo) (string, []byte, error) {
	if !server.TLS {
		advertised := false
		for _, mechanism := range server.Auth {
			if mechanism == "PLAIN" {
				advertised = true
				break
			}
		}
		if !advertised {
			if a.tryNTLM {
				a.auth = NTLMAuth(server.Name, a.username, a.password, "")
				return a.auth.Start(server)
			}

			return "", nil, errors.New("unencrypted connection")
		}
	}

	if a.tryNTLM {
		advertised := false
		for _, mechanism := range server.Auth {
			if mechanism == "PLAIN" {
				advertised = true
				break
			}
		}

		if !advertised {
			for _, mechanism := range server.Auth {
				if mechanism == "NTLM" {
					a.auth = NTLMAuth(server.Name, a.username, a.password, "")
					return a.auth.Start(server)
				}
			}
		}
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
	}
	resp := []byte(a.identity + "\x00" + a.username + "\x00" + a.password)
	return "PLAIN", resp, nil
}

func (a *plainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if a.auth != nil {
		return a.auth.Next(fromServer, more)
	}

	if more {
		// We've already sent everything.
		return nil, errors.New("unexpected server challenge")
	}
	return nil, nil
}

type cramMD5Auth struct {
	username, secret string
}

// CRAMMD5Auth returns an Auth that implements the CRAM-MD5 authentication
// mechanism as defined in RFC 2195.
// The returned Auth uses the given username and secret to authenticate
// to the server using the challenge-response mechanism.
func CRAMMD5Auth(username, secret string) Auth {
	return &cramMD5Auth{username, secret}
}

func (a *cramMD5Auth) Start(server *ServerInfo) (string, []byte, error) {
	return "CRAM-MD5", nil, nil
}

func (a *cramMD5Auth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		d := hmac.New(md5.New, []byte(a.secret))
		d.Write(fromServer)
		s := make([]byte, 0, d.Size())
		return []byte(fmt.Sprintf("%s %x", a.username, d.Sum(s))), nil
	}
	return nil, nil
}

// PlainAuth returns an Auth that implements the PLAIN authentication
// mechanism as defined in RFC 4616.
// The returned Auth uses the given username and password to authenticate
// on TLS connections to host and act as identity. Usually identity will be
// left blank to act as username.
func NTLMAuth(host, user, password, workstation string) *ntlmAuth {
	domanAndUsername := strings.SplitN(user, `\`, 2)
	if len(domanAndUsername) != 2 {
		return &ntlmAuth{initErr: errors.New(`Wrong format of username. The required format is 'domain\username'`)}
	}

	a := NTLMSSP{
		Domain:      domanAndUsername[0],
		UserName:    domanAndUsername[1],
		Password:    password,
		Workstation: workstation,
	}

	return &ntlmAuth{
		NTLMSSP: a,
		Host:    host,
	}
}

// NTLMAuth implements smtp.Auth. The authentication mechanism.
type ntlmAuth struct {
	NTLMSSP
	Host    string
	initErr error
}

func (n *ntlmAuth) Start(server *ServerInfo) (string, []byte, error) {
	if n.initErr != nil {
		return "", nil, n.initErr
	}
	if !server.TLS {
		var isNTLM bool
		for _, mechanism := range server.Auth {
			isNTLM = isNTLM || mechanism == "NTLM"
		}

		if !isNTLM {
			return "", nil, errors.New("mail: unknown authentication type:" + fmt.Sprintln(server.Auth))
		}
	}

	//if server.Name != n.Host {
	//	return "", nil, errors.New("mail: wrong host name")
	//}

	return "NTLM", nil, nil
}

func (n *ntlmAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	switch {
	case bytes.Equal(fromServer, []byte("NTLM supported")):
		return n.InitialBytes()
	default:
		maxLen := base64.StdEncoding.DecodedLen(len(fromServer))

		dst := make([]byte, maxLen)
		resultLen, err := base64.StdEncoding.Decode(dst, fromServer)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Decode base64 error: %s", err.Error()))
		}

		var challengeMessage []byte
		if maxLen == resultLen {
			challengeMessage = dst
		} else {
			challengeMessage = make([]byte, resultLen, resultLen)
			copy(challengeMessage, dst)
		}

		return n.NextBytes(challengeMessage)
	}
}
