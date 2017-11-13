package delayed_job

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/runner-mei/delayed_job/smtp"
)

var tryNTLM = os.Getenv("try_ntlm") == "true"
var BlatExecute = os.Getenv("blat_path")

var default_mail_subject_encoding string
var default_mail_auth_type = flag.String("mail.auth.type", "", "the auth type of smtp")
var default_mail_auth_user = flag.String("mail.auth.user", "", "the auth user of smtp")
var default_mail_auth_identity = flag.String("mail.auth.identity", "", "the auth identity of smtp")
var default_mail_auth_password = flag.String("mail.auth.password", "", "the auth password of smtp")
var default_mail_auth_host = flag.String("mail.auth.host", "", "the auth host of smtp")

func init() {
	flag.StringVar(&default_mail_subject_encoding, "mail.subject_encoding", "gb2312_base64", "")
}

var defaultSmtpServer = flag.String("mail.smtp_server", "", "the address of smtp server")
var default_mail_address = flag.String("mail.from", "", "the from address of mail")

type mailHandler struct {
	smtpServer string
	message    *MailMessage

	authType    string
	identity    string
	user        string
	password    string
	host        string
	removeFiles []string
	closers     []io.Closer
}

func toAddressListString(addresses []*mail.Address) string {
	var buf bytes.Buffer
	for _, a := range addresses {
		buf.WriteString(a.Address)
		buf.WriteString(",")
	}
	if buf.Len() > 0 {
		buf.Truncate(buf.Len() - 1)
	}
	return buf.String()
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
		scan := bufio.NewScanner(strings.NewReader(s))

		results := make([]*mail.Address, 0, 4)
		for scan.Scan() {
			bs := scan.Bytes()
			if len(bs) == 0 {
				continue
			}

			bs = bytes.TrimSpace(bs)
			if len(bs) == 0 {
				continue
			}

			addr, e := mail.ParseAddressList(string(bs))
			if nil != e {
				return nil, errors.New("'" + nm + "' is invalid - " + e.Error())
			}
			results = append(results, addr...)
		}
		return results, nil
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

	var authType string
	var identity string
	var password string
	var host string
	var user string = stringWithDefault(params, "user", "")
	if 0 == len(user) {
		authType = *default_mail_auth_type
		user = *default_mail_auth_user
		identity = *default_mail_auth_identity
		password = *default_mail_auth_password
		host = *default_mail_auth_host
	} else {
		authType = stringWithDefault(params, "auth_type", "plain")
		if 0 == len(authType) {
			return nil, errors.New("'auth_type' is required")
		}
		identity = stringWithDefault(params, "identity", "")
		password = stringWithDefault(params, "password", "")
		host = stringWithDefault(params, "host", "")
	}

	smtpServer := stringWithDefault(params, "smtp_server", "")
	if 0 == len(smtpServer) {
		smtpServer = *defaultSmtpServer
	}

	if 0 == len(host) {
		idx := strings.IndexRune(smtpServer, ':')
		if -1 != idx {
			host = smtpServer[0:idx]
		}
	}

	var contentText string
	var contentHtml string
	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		contentText = stringWithDefault(params, "content_text", "")
		contentHtml = stringWithDefault(params, "content_html", "")

		if "" == contentHtml && "" == contentText {
			return nil, errors.New("'content' is required.")
		}
	}

	subject := stringWithDefault(params, "subject", "")
	if 0 == len(subject) {
		return nil, errors.New("'subject' is required.")
	}

	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)
		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}

		var e error
		subject, e = genText(subject, args)
		if nil != e {
			return nil, e
		}
		if "" != content {
			content, e = genText(content, args)
			if nil != e {
				return nil, e
			}
		}
		if "" != contentHtml {
			contentHtml, e = genText(contentHtml, args)
			if nil != e {
				return nil, e
			}
		}

		if "" != contentText {
			contentText, e = genText(contentText, args)
			if nil != e {
				return nil, e
			}
		}
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

	users, e := addressesWith(params, "to_mail_addresses")
	if nil != e {
		return nil, e
	}
	to, e := addressesWith(params, "to_address")
	if nil != e {
		return nil, e
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

	if "" != content {
		contentType := stringWithDefault(params, "content_type", "")
		switch contentType {
		case "html":
			contentHtml = content
		case "text", "":
			contentText = content
		default:
			return nil, errors.New("'" + contentType + "' is unsupported.")
		}
	}
	var removeFiles []string
	var closers []io.Closer
	var attachments []Attachment
	if args, ok := params["attachments"]; ok {
		if ar, ok := args.([]interface{}); ok {
			for _, a := range ar {
				var nm, file string
				var is_removed bool

				if param, ok := a.(map[string]interface{}); ok {
					nm = stringWithDefault(param, "name", "")
					file = stringWithDefault(param, "file", "")
					is_removed = boolWithDefault(param, "is_removed", false)
				} else {
					file = fmt.Sprint(a)
					nm = filepath.Base(file)
				}

				if "" == file {
					continue
				}

				if "" == nm {
					nm = filepath.Base(file)
				}

				if is_removed {
					removeFiles = append(removeFiles, file)
				}

				f, e := os.Open(file)
				if nil != e {
					return nil, e
				}

				attachments = append(attachments, Attachment{Name: nm, Content: f})

				closers = append(closers, f)
			}
		}
	}

	return &mailHandler{smtpServer: smtpServer,
		authType:    authType,
		identity:    identity,
		user:        user,
		password:    password,
		host:        host,
		removeFiles: removeFiles,
		closers:     closers,
		message: &MailMessage{From: *from,
			To:          to,
			Cc:          cc,
			Bcc:         bcc,
			Subject:     subject,
			ContentText: contentText,
			ContentHtml: contentHtml,
			Attachments: attachments}}, nil
}

func (self *mailHandler) Perform() error {
	if 0 == len(self.message.To) {
		return nil
	}

	close := func() {
		if len(self.closers) > 0 {
			for _, closer := range self.closers {
				closer.Close()
			}
			self.closers = nil
		}

		for _, nm := range self.removeFiles {
			if e := os.Remove(nm); nil != e {
				log.Println("[warn] [mail] remove file - '"+nm+"',", e)
			}
		}
	}
	defer close()

	if BlatExecute != "" {
		cmd := exec.Command(BlatExecute,
			"-from", self.message.From.Address,
			"-server", self.smtpServer,
			"-f", self.message.From.Address,
			"-u", self.user,
			"-pw", self.password)

		if len(self.message.To) > 0 {
			cmd.Args = append(cmd.Args, "-to", toAddressListString(self.message.To))
		}
		if len(self.message.Cc) > 0 {
			cmd.Args = append(cmd.Args, "-cc", toAddressListString(self.message.Cc))
		}
		if len(self.message.Bcc) > 0 {
			cmd.Args = append(cmd.Args, "-bcc", toAddressListString(self.message.Bcc))
		}
		cmd.Args = append(cmd.Args, "-subject", self.message.Subject)

		if self.message.ContentHtml == "" {
			cmd.Args = append(cmd.Args, "-body", self.message.ContentText)
		} else {
			cmd.Args = append(cmd.Args, "-html", self.message.ContentHtml)
			if self.message.ContentHtml == "" {
				cmd.Args = append(cmd.Args, "-alttext", self.message.ContentText)
			}
		}
		for _, attachment := range self.message.Attachments {
			if f, ok := attachment.Content.(*os.File); ok {
				filename := f.Name()

				cmd.Args = append(cmd.Args, "-attach", filename)
			}
		}

		timer := time.AfterFunc(10*time.Minute, func() {
			defer recover()
			cmd.Process.Kill()
		})
		output, e := cmd.CombinedOutput()
		timer.Stop()

		if nil != e {
			fmt.Println(cmd.Path, cmd.Args)
			return errors.New(string(output))
		}

		log.Println("################\r\n" + string(output) + "\r\n################")
		// if !bytes.Contains(output, []byte(excepted)) {
		// 	return errors.New(string(output))
		// }
		return nil
	}

	var auth smtp.Auth
	if "" != self.password {
		switch strings.ToLower(self.authType) {
		case "":
			if 0 != len(self.password) {
				if 0 == len(self.user) {
					self.user = toMailString(&self.message.From)

					if 0 == len(self.user) {
						return errors.New("user is missing")
					}
				}
				auth = smtp.PlainAuth(self.identity, self.user, self.password, self.host, true)
			}
		case "login":
			auth = smtp.LoginAuth(self.user, self.password)
		case "plain":
			if 0 == len(self.user) {
				self.user = toMailString(&self.message.From)
				if 0 == len(self.user) {
					return errors.New("user is missing")
				}
			}
			if 0 == len(self.host) {
				self.host = self.smtpServer
			}
			auth = smtp.PlainAuth(self.identity, self.user, self.password, self.host, tryNTLM)
		case "cram-md5":
			auth = smtp.CRAMMD5Auth(self.user, self.password)
		case "ntlm":
			auth = smtp.NTLMAuth("", self.user, self.password, "")
		default:
			return errors.New("unsupported auth type - " + self.authType)
		}
	}
	if e := self.message.Send(self.smtpServer, auth); nil != e {
		return e
	}
	return nil
}

func init() {
	Handlers["mail"] = newMailHandler
	Handlers["mail_command"] = newMailHandler
	Handlers["smtp"] = newMailHandler
	Handlers["smtp_command"] = newMailHandler
}
