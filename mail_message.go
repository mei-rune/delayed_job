package delayed_job

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/runner-mei/delayed_job/smtp"
)

const crlf = "\r\n"

type MailMessage struct {
	From        mail.Address // if From.Address is empty, Config.DefaultFrom will be used
	To          []*mail.Address
	Cc          []*mail.Address
	Bcc         []*mail.Address
	Subject     string
	Content     string
	ContentType string
}

func toMailString(addr *mail.Address) string {
	if 0 == len(addr.Name) {
		return addr.Address
	}
	return addr.String()
}

// http://tools.ietf.org/html/rfc822
// http://tools.ietf.org/html/rfc2821
func (self *MailMessage) Bytes() []byte {
	from := toMailString(&self.From)

	buf := bytes.NewBuffer(make([]byte, 0, 10240))
	write := func(what string, recipients []*mail.Address) {
		if nil == recipients {
			return
		}
		if 0 == len(recipients) {
			return
		}
		for i := range recipients {
			if 0 == i {
				buf.WriteString(what)
			} else {
				buf.WriteString(", ")
			}
			buf.WriteString(toMailString(recipients[i]))
		}
		buf.WriteString(crlf)
	}

	fmt.Fprintf(buf, "From: %s%s", from, crlf)
	write("To: ", self.To)
	write("Cc: ", self.Cc)
	write("Bcc: ", self.Bcc)
	boundary := self.GetBoundary()
	fmt.Fprintf(buf, "Date: %s%s", time.Now().UTC().Format(time.RFC822), crlf)
	fmt.Fprintf(buf, "Subject: %s%s", self.Subject, crlf)
	fmt.Fprintf(buf, "Content-Type: multipart/alternative; boundary=%s%s%s", boundary, crlf, crlf)

	fmt.Fprintf(buf, "%s%s", "--"+boundary, crlf)

	switch self.ContentType {
	case "html":
		fmt.Fprintf(buf, "Content-Type: text/html; charset=UTF-8%s", crlf)
	case "text", "":
		fmt.Fprintf(buf, "Content-Type: text/plain; charset=UTF-8%s", crlf)
	default:
		fmt.Fprintf(buf, "Content-Type: %s%s", self.ContentType, crlf)
	}

	fmt.Fprintf(buf, "%s%s%s%s", crlf, self.Content, crlf, crlf)
	fmt.Fprintf(buf, "%s%s", "--"+boundary+"--", crlf)

	return buf.Bytes()
}

func (self *MailMessage) Send(smtp_server string, auth smtp.Auth) error {
	if nil == self.To || 0 == len(self.To) {
		return errors.New("'to_address' is missing.")
	}

	if 0 == len(smtp_server) {
		smtp_server = *default_smtp_server
		if 0 == len(smtp_server) {
			return errors.New("'smtp_server' is missing or default 'smtp_server' is not set.")
		}
	}

	if !strings.Contains(smtp_server, ":") {
		smtp_server += ":25"
	}

	to := make([]string, len(self.To))
	for i := range self.To {
		to[i] = toMailString(self.To[i])
	}

	from := toMailString(&self.From)
	if 0 == len(from) {
		return errors.New("'from_address' is missing or default 'from_address' is not set.")
	}

	//fmt.Println(string(self.Bytes()))
	return smtp.SendMail(smtp_server, auth, from, to, self.Bytes())
}

// Generate aun unique boundary value
func (self *MailMessage) GetBoundary() string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s", time.Now().Nanosecond()))
	return fmt.Sprintf("%x", h.Sum(nil))
}
