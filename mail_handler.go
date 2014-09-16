package delayed_job

import (
	"bytes"
	"errors"
	"flag"
	"net/mail"
	"net/smtp"
	"strings"
	"text/template"
)

var default_mail_auth_type = flag.String("mail.auth.type", "", "the auth type of smtp")
var default_mail_auth_user = flag.String("mail.auth.user", "", "the auth user of smtp")
var default_mail_auth_identity = flag.String("mail.auth.identity", "", "the auth identity of smtp")
var default_mail_auth_password = flag.String("mail.auth.password", "", "the auth password of smtp")
var default_mail_auth_host = flag.String("mail.auth.host", "", "the auth host of smtp")

var default_smtp_server = flag.String("mail.smtp_server", "", "the address of smtp server")
var default_mail_address = flag.String("mail.from", "", "the from address of mail")

type mailHandler struct {
	smtp_server string
	message     *MailMessage

	auth_type string
	identity  string
	user      string
	password  string
	host      string
}

func addressesWith(params map[string]interface{}, nm string) ([]*mail.Address, error) {
	o, ok := params[nm]
	if !ok {
		return nil, nil
	}
	if nil == o {
		return nil, nil
	}
	if s, ok := o.(string); ok {
		if 0 == len(s) {
			return nil, nil
		}
		addr, e := mail.ParseAddressList(s)
		if nil != e {
			return nil, errors.New("'" + nm + "' is invalid - " + e.Error())
		}
		return addr, nil
	}

	if m, ok := o.(map[string]interface{}); ok {
		address := stringWithDefault(m, "address", "")
		if 0 == len(address) {
			return nil, errors.New("'" + nm + "' is invalid.")
		}
		return []*mail.Address{&mail.Address{Name: stringWithDefault(m, "name", ""), Address: address}}, nil
	}

	if m, ok := o.([]interface{}); ok {
		addresses := make([]*mail.Address, len(m))
		var e error
		for i := range m {
			addresses[i], e = toAddress(m[i], nm)
			if nil != e {
				return nil, e
			}
		}
		return addresses, nil
	}
	return nil, errors.New("'" + nm + "' is invalid.")
}

func addressWith(params map[string]interface{}, nm string) (*mail.Address, error) {
	o, ok := params[nm]
	if !ok {
		return nil, nil
	}
	if nil == o {
		return nil, nil
	}
	return toAddress(o, nm)
}

func toAddress(o interface{}, nm string) (*mail.Address, error) {
	if s, ok := o.(string); ok {
		if 0 == len(s) {
			return nil, nil
		}
		addr, e := mail.ParseAddress(s)
		if nil != e {
			return nil, errors.New("'" + nm + "' is invalid - " + e.Error())
		}
		return addr, nil
	}

	if m, ok := o.(map[string]interface{}); ok {
		address := stringWithDefault(m, "address", "")
		if 0 == len(address) {
			return nil, errors.New("'" + nm + "' is invalid.")
		}
		return &mail.Address{Name: stringWithDefault(m, "name", ""), Address: address}, nil
	}
	return nil, errors.New("'" + nm + "' is invalid.")
}

func newMailHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	var auth_type string
	var identity string
	var password string
	var host string
	var user string = stringWithDefault(params, "user", "")
	if 0 == len(user) {
		auth_type = *default_mail_auth_type
		user = *default_mail_auth_user
		identity = *default_mail_auth_identity
		password = *default_mail_auth_password
		host = *default_mail_auth_host
	} else {
		auth_type = stringWithDefault(params, "auth_type", "plain")
		if 0 == len(auth_type) {
			return nil, errors.New("'auth_type' is required.")
		}
		identity = stringWithDefault(params, "identity", "")
		password = stringWithDefault(params, "password", "")
		host = stringWithDefault(params, "host", "")
	}

	smtp_server := stringWithDefault(params, "smtp_server", "")
	if 0 == len(smtp_server) {
		smtp_server = *default_smtp_server
	}

	if 0 == len(host) {
		idx := strings.IndexRune(smtp_server, ':')
		if -1 != idx {
			host = smtp_server[0:idx]
		}
	}

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required.")
	}

	if args, ok := params["arguments"]; ok && strings.Contains(content, "{{") {
		t, e := template.New("default").Parse(content)
		if nil != e {
			return nil, errors.New("create template failed, " + e.Error())
		}
		var buffer bytes.Buffer
		e = t.Execute(&buffer, args)
		if nil != e {
			return nil, errors.New("execute template failed, " + e.Error())
		}
		content = buffer.String()
	}

	subject := stringWithDefault(params, "subject", "")
	if 0 == len(subject) {
		return nil, errors.New("'subject' is required.")
	}

	if args, ok := params["arguments"]; ok && strings.Contains(subject, "{{") {
		t, e := template.New("default").Parse(subject)
		if nil != e {
			return nil, errors.New("create template failed, " + e.Error())
		}
		var buffer bytes.Buffer
		e = t.Execute(&buffer, args)
		if nil != e {
			return nil, errors.New("execute template failed, " + e.Error())
		}
		subject = buffer.String()
	}

	from, e := addressWith(params, "from_address")
	if nil != e {
		return nil, e
	}
	if nil == from {
		from, e = toAddress(*default_mail_address, "from_address")
		if nil != e {
			return nil, e
		}
		if nil == from {
			return nil, errors.New("'from_address' is required.")
		}
	}

	users, e := addressesWith(params, "users")
	if nil != e {
		return nil, e
	}
	to, e := addressesWith(params, "to_address")
	if nil != e {
		return nil, e
	}
	if 0 == len(to) && 0 == len(users) {
		return nil, errors.New("'to_address' is missing.")
	}
	if 0 == len(to) {
		to = users
	} else if 0 != len(users) {
		to = append(to, users...)
	}

	cc, e := addressesWith(params, "cc_address")
	if nil != e {
		return nil, e
	}

	bcc, e := addressesWith(params, "bcc_address")
	if nil != e {
		return nil, e
	}

	contentType := stringWithDefault(params, "content_type", "")

	return &mailHandler{smtp_server: smtp_server,
		auth_type: auth_type,
		identity:  identity,
		user:      user,
		password:  password,
		host:      host,
		message: &MailMessage{From: *from,
			To:          to,
			Cc:          cc,
			Bcc:         bcc,
			Subject:     subject,
			Content:     content,
			ContentType: contentType}}, nil
}

func (self *mailHandler) Perform() error {
	var auth smtp.Auth = nil
	switch self.auth_type {
	case "":
		if 0 != len(self.password) {
			if 0 == len(self.user) {
				self.user = toMailString(&self.message.From)

				if 0 == len(self.user) {
					return errors.New("user is missing.")
				}
			}
			auth = smtp.PlainAuth(self.identity, self.user, self.password, self.host)
		}
	case "plain", "PLAIN":
		if 0 == len(self.user) {
			self.user = toMailString(&self.message.From)
			if 0 == len(self.user) {
				return errors.New("user is missing.")
			}
		}
		if 0 == len(self.host) {
			self.host = self.smtp_server
		}
		auth = smtp.PlainAuth(self.identity, self.user, self.password, self.host)
	case "cram-md5", "CRAM-MD5":
		auth = smtp.CRAMMD5Auth(self.user, self.password)
	default:
		return errors.New("unsupported auth type - " + self.auth_type)
	}
	return self.message.Send(self.smtp_server, auth)
}

func init() {
	Handlers["mail"] = newMailHandler
	Handlers["mail_command"] = newMailHandler
	Handlers["smtp"] = newMailHandler
	Handlers["smtp_command"] = newMailHandler
}
