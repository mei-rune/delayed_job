package delayed_job

import (
	"flag"
	"testing"
)

var test_mail_to = flag.String("test.mail_to", "", "the address of mail")
var test_mail_from = flag.String("test.mail_from", "", "the address of mail")
var test_smtp_server = flag.String("test.smtp_server", "", "the address of smtp server")
var test_mail_auth_password = flag.String("test.mail.password", "", "the address of smtp server")

func TestMailHandlerParameterIsError(t *testing.T) {
	_, e := newMailHandler(nil, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "ctx is nil" != e.Error() {
		t.Error("excepted error is 'ctx is nil', but actual is", e)
	}

	_, e = newMailHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}

	_, e = newMailHandler(map[string]interface{}{}, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'content' is required." != e.Error() {
		t.Error("excepted error is ['content' is required.], but actual is", e)
	}
}

func TestMailHandler(t *testing.T) {
	// *default_mail_auth_type = ""
	// *default_mail_auth_user = ""
	// *default_mail_auth_identity = ""
	// *default_mail_auth_password = ""
	// *default_mail_auth_host = ""

	if "" == *test_smtp_server {
		t.Skip("please set 'test.mail_to', 'test.mail_from' and 'test.smtp_server'")
		return
	}

	*default_mail_auth_user = *test_mail_from
	*default_mail_auth_password = *test_mail_auth_password
	*default_smtp_server = *test_smtp_server
	*default_mail_address = *test_mail_from

	handler, e := newMailHandler(map[string]interface{}{}, map[string]interface{}{"to": *test_mail_to, "subject": "this is test subject!", "content": `
		this is a test mail!
		`})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}

func TestMailHandlerWithArguments(t *testing.T) {
	// *default_mail_auth_type = ""
	// *default_mail_auth_user = ""
	// *default_mail_auth_identity = ""
	// *default_mail_auth_password = ""
	// *default_mail_auth_host = ""

	if "" == *test_smtp_server {
		t.Skip("please set 'test.mail_to', 'test.mail_from' and 'test.smtp_server'")
		return
	}

	*default_mail_auth_user = *test_mail_from
	*default_mail_auth_password = *test_mail_auth_password
	*default_smtp_server = *test_smtp_server
	*default_mail_address = *test_mail_from

	handler, e := newMailHandler(map[string]interface{}{}, map[string]interface{}{"arguments": map[string]interface{}{"client": "mei"},
		"to": *test_mail_to, "subject": "this is test subject!", "content": `
	{{.client}}, this is a test mail!
		`})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil != e {
		t.Error(e)
		return
	}
}
