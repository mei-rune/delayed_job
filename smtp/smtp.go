// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package smtp implements the Simple Mail Transfer Protocol as defined in RFC 5321.
// It also implements the following extensions:
//
//	8BITMIME  RFC 1652
//	AUTH      RFC 2554
//	STARTTLS  RFC 3207
//
// Additional extensions may be handled by clients.
package smtp

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
)

var skipAuthError = os.Getenv("smtp_skip_auth_error") == "true"

func EnableDebug()  {}
func DisableDebug() {}

type TLSMethod int

const (
	TlsAuto    TLSMethod = 0
	TlsConnect TLSMethod = 1
	TlsNever   TLSMethod = 2
)

func UseTLS(s string) TLSMethod {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "false", "never":
		return TlsNever
	case "true", "always":
		return TlsConnect
	}

	return TlsAuto
}

// A Client represents a client connection to an SMTP server.
type Client struct {
	Output io.Writer

	// Text is the textproto.Conn used by the Client. It is exported to allow for
	// clients to add extensions.
	Text *textproto.Conn
	// keep a reference to the connection so it can be used to create a TLS
	// connection later
	conn io.ReadWriteCloser
	// whether the Client is using TLS
	tls        bool
	useTLS     TLSMethod
	serverName string
	// map of supported extensions
	ext map[string]string
	// supported auth mechanisms
	auth       []string
	localName  string // the name to use in HELO/EHLO
	didHello   bool   // whether we've said HELO/EHLO
	helloError error  // the error from the hello
}

func (c *Client) IsTLS() bool {
	return c.tls
}

// Dial returns a new Client connected to an SMTP server at addr.
// The addr must include a port number.
func Dial(addr string, useTLS TLSMethod, useFQDN bool, output io.Writer) (*Client, error) {
	fprintln(output, "===========", addr, "===========")

	host, port, _ := net.SplitHostPort(addr)
	if useTLS == TlsConnect || (useTLS == TlsAuto && port == "587") {
		config := &tls.Config{ServerName: "",
			InsecureSkipVerify: true}
		conn, err := tls.Dial("tcp", addr, config)
		if err == nil {
			client, err := NewClient(conn, host, output, useFQDN)
			if err == nil {
				fprintln(output, "connect with tls")
				client.useTLS = useTLS
				client.tls = true
				return client, nil
			}
		}
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(conn, host, output, useFQDN)
	if err == nil {
		client.useTLS = useTLS
	}
	return client, err
}

// NewClient returns a new Client using an existing connection and host as a
// server name to be used when authenticating.
func NewClient(conn io.ReadWriteCloser, host string, output io.Writer, useFQDN bool) (*Client, error) {
	text := textproto.NewConn(conn)

	_, _, err := text.ReadResponse(220)
	if err != nil {
		text.Close()
		if err == io.EOF {
			return nil, errors.New("connect to mail server, but read welcome message error")
		}
		return nil, err
	}
	net.LookupHost(host)
	c := &Client{Output: output, Text: text, conn: conn, serverName: host, localName: "localhost"}
	if useFQDN {
		c.localName = GetFQDN()
	}

	_, c.tls = conn.(*tls.Conn)
	return c, nil
}

func (c *Client) println(args ...interface{}) {
	fprintln(c.Output, args...)
}

func fprintln(output io.Writer, args ...interface{}) {
	if output != nil {
		fmt.Fprintln(output, args...)
	} else {
		fmt.Println(args...)
	}
}

// Close closes the connection.
func (c *Client) Close() error {
	c.println("C: CLOSED")

	return c.Text.Close()
}

// hello runs a hello exchange if needed.
func (c *Client) hello() error {
	if !c.didHello {
		c.didHello = true
		err := c.ehlo()
		if err != nil {
			c.helloError = c.helo()
		}
	}
	return c.helloError
}

// Hello sends a HELO or EHLO to the server as the given host name.
// Calling this method is only necessary if the client needs control
// over the host name used.  The client will introduce itself as "localhost"
// automatically otherwise.  If Hello is called, it must be called before
// any of the other methods.
func (c *Client) Hello(localName string) error {
	if c.didHello {
		return errors.New("smtp: Hello called after other methods")
	}
	c.localName = localName
	return c.hello()
}

// cmd is a convenience function that sends a command and returns the response
func (c *Client) cmd(expectCode int, format string, args ...interface{}) (int, string, error) {
	c.println("C:", fmt.Sprintf(format, args...))

	id, err := c.Text.Cmd(format, args...)
	if err != nil {
		if err == io.EOF {
			return 0, "", errors.New("ERROR: " + fmt.Sprintf(format, args...))
		}
		return 0, "", err
	}
	c.Text.StartResponse(id)
	defer c.Text.EndResponse(id)
	code, msg, err := c.Text.ReadResponse(expectCode)
	c.println("S:", code, msg, err)
	if err == io.EOF {
		return code, msg, errors.New("ERROR: " + fmt.Sprintf(format, args...))
	}
	return code, msg, err
}

// helo sends the HELO greeting to the server. It should be used only when the
// server does not support ehlo.
func (c *Client) helo() error {
	c.ext = nil
	_, _, err := c.cmd(250, "HELO %s", c.localName)
	return err
}

// ehlo sends the EHLO (extended hello) greeting to the server. It
// should be the preferred greeting for servers that support it.
func (c *Client) ehlo() error {
	_, msg, err := c.cmd(250, "EHLO %s", c.localName)
	if err != nil {
		return err
	}
	ext := make(map[string]string)
	extList := strings.Split(msg, "\n")
	if len(extList) > 1 {
		extList = extList[1:]
		for _, line := range extList {
			args := strings.SplitN(line, " ", 2)
			if len(args) > 1 {
				ext[args[0]] = args[1]
			} else {
				ext[args[0]] = ""
			}
		}
	}
	if mechs, ok := ext["AUTH"]; ok {
		c.auth = strings.Split(mechs, " ")
	}
	c.ext = ext
	return err
}

// StartTLS sends the STARTTLS command and encrypts all further communication.
// Only servers that advertise the STARTTLS extension support this function.
func (c *Client) StartTLS(config *tls.Config) error {
	if c.tls {
		return errors.New("conn is already tls")
	}

	conn, ok := c.conn.(net.Conn)
	if !ok {
		return nil
	}

	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(220, "STARTTLS")
	if err != nil {
		return err
	}
	c.conn = tls.Client(conn, config)
	c.Text = textproto.NewConn(c.conn)
	c.tls = true
	return c.ehlo()
}

// Verify checks the validity of an email address on the server.
// If Verify returns nil, the address is valid. A non-nil return
// does not necessarily indicate an invalid address. Many servers
// will not verify addresses for security reasons.
func (c *Client) Verify(addr string) error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(250, "VRFY %s", addr)
	return err
}

// Auth authenticates a client using the provided authentication mechanism.
// A failed authentication closes the connection.
// Only servers that advertise the AUTH extension support this function.
func (c *Client) Auth(a Auth) error {
	if err := c.hello(); err != nil {
		return err
	}
	encoding := base64.StdEncoding
	mech, resp, err := a.Start(&ServerInfo{c.serverName, c.tls, c.auth})
	if err != nil {
		c.Quit()
		return err
	}
	resp64 := make([]byte, encoding.EncodedLen(len(resp)))
	encoding.Encode(resp64, resp)
	code, msg64, err := c.cmd(0, "AUTH %s %s", mech, resp64)
	for err == nil {
		var msg []byte
		switch code {
		case 334:
			if msg64 == "NTLM supported" {
				msg = []byte(msg64)
			} else {
				msg, err = encoding.DecodeString(msg64)
			}
		case 235:
			// the last message isn't base64 because it isn't a challenge
			msg = []byte(msg64)
		default:
			err = &textproto.Error{Code: code, Msg: msg64}
		}
		if err == nil || skipAuthError {
			if err != nil {
				msg = []byte("username:")
				code = 334
			}

			resp, err = a.Next(msg, code == 334)
		}
		if err != nil {
			// abort the AUTH
			// c.cmd(501, "*")
			c.Quit()
			break
		}
		if resp == nil {
			break
		}
		resp64 = make([]byte, encoding.EncodedLen(len(resp)))
		encoding.Encode(resp64, resp)
		code, msg64, err = c.cmd(0, string(resp64))
	}
	return err
}

// Mail issues a MAIL command to the server using the provided email address.
// If the server supports the 8BITMIME extension, Mail adds the BODY=8BITMIME
// parameter.
// This initiates a mail transaction and is followed by one or more Rcpt calls.
func (c *Client) Mail(from string) error {
	if err := c.hello(); err != nil {
		return err
	}
	cmdStr := "MAIL FROM:<%s>"
	if c.ext != nil {
		if _, ok := c.ext["8BITMIME"]; ok {
			cmdStr += " BODY=8BITMIME"
		}
	}
	_, _, err := c.cmd(250, cmdStr, from)
	return err
}

// Rcpt issues a RCPT command to the server using the provided email address.
// A call to Rcpt must be preceded by a call to Mail and may be followed by
// a Data call or another Rcpt call.
func (c *Client) Rcpt(to string) error {
	_, _, err := c.cmd(25, "RCPT TO:<%s>", to)
	return err
}

type dataCloser struct {
	c *Client
	io.WriteCloser
}

func (d *dataCloser) Close() error {
	d.WriteCloser.Close()
	_, _, err := d.c.Text.ReadResponse(250)
	return err
}

// Data issues a DATA command to the server and returns a writer that
// can be used to write the data. The caller should close the writer
// before calling any more methods on c.
// A call to Data must be preceded by one or more calls to Rcpt.
func (c *Client) Data() (io.WriteCloser, error) {
	_, _, err := c.cmd(354, "DATA")
	if err != nil {
		return nil, err
	}
	return &dataCloser{c, c.Text.DotWriter()}, nil
}

var testHookStartTLS func(*tls.Config) // nil, except for tests

// SendMail connects to the server at addr, switches to TLS if
// possible, authenticates with the optional mechanism a if possible,
// and then sends an email from address from, to addresses to, with
// message msg.
func SendMail(addr string, a Auth, from string, to []string, msg []byte, useTLS TLSMethod, useFQDN bool, output io.Writer) error {
	c, err := Dial(addr, useTLS, useFQDN, output)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.hello(); err != nil {
		return err
	}

	if c.useTLS == TlsAuto && !c.tls {
		if ok, _ := c.Extension("STARTTLS"); ok {
			config := &tls.Config{ServerName: c.serverName,
				InsecureSkipVerify: true}
			if testHookStartTLS != nil {
				testHookStartTLS(config)
			}
			if err = c.StartTLS(config); err != nil {
				return err
			}
		}
	}
	if a != nil && c.ext != nil {
		if _, ok := c.ext["AUTH"]; ok {
			if err = c.Auth(a); err != nil {
				return err
			}
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

// Extension reports whether an extension is support by the server.
// The extension name is case-insensitive. If the extension is supported,
// Extension also returns a string that contains any parameters the
// server specifies for the extension.
func (c *Client) Extension(ext string) (bool, string) {
	if err := c.hello(); err != nil {
		return false, ""
	}
	if c.ext == nil {
		return false, ""
	}
	ext = strings.ToUpper(ext)
	param, ok := c.ext[ext]
	return ok, param
}

// Reset sends the RSET command to the server, aborting the current mail
// transaction.
func (c *Client) Reset() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(250, "RSET")
	return err
}

// Quit sends the QUIT command and closes the connection to the server.
func (c *Client) Quit() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(221, "QUIT")
	if err != nil {
		return err
	}
	return c.Text.Close()
}
