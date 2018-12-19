package delayed_job

import (
	"net/mail"
	"strings"
	"testing"

	"github.com/runner-mei/delayed_job/smtp"
)

func TestMailMessageTextAndHtml(t *testing.T) {
	if "" == *defaultSmtpServer {
		t.Skip("please set 'test.mail_to', 'mail.from' and 'mail.smtp_server'")
		return
	}

	msg := &MailMessage{
		From: mail.Address{
			Name:    "发件人的名字",
			Address: *default_mail_address}, // if From.Address is empty, Config.DefaultFrom will be used
		To: []*mail.Address{&mail.Address{
			Name:    "收件人的名字",
			Address: *test_mail_to}},
		Subject:     "这是一个 增 消息",
		ContentText: "这是一个Text消息",
		ContentHtml: `<!doctype html>
		<html lang="en">
		 <head>
		  <meta charset="UTF-8">
		  <title>test</title>
		 </head>
		 <body>
		  这是一个 Html 消息
		 </body>
		</html>
		`,
		Attachments: []Attachment{
			{Name: "中文名的附件.txt", Content: strings.NewReader("aaaaaoooo")},
		}}

	// bs, e := msg.Bytes()
	// if nil != e {
	// 	t.Error(e)
	// 	return
	// }
	// t.Log(string(bs))

	if e := msg.Send(*defaultSmtpServer, smtp.PlainAuth(*default_mail_auth_identity,
		*default_mail_address,
		*default_mail_auth_password,
		*default_mail_auth_host,
		tryNTLM),
		*default_mail_useFQDN,
		*default_mail_noTLS); nil != e {
		t.Error(e)
	}
}
