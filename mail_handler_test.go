package delayed_job

import (
	"flag"
	"testing"
)

var test_mail_to = flag.String("test.mail_to", "", "the address of mail")

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
	if "" == *defaultSmtpServer {
		t.Skip("please set 'test.mail_to', 'mail.from' and 'mail.smtp_server'")
		return
	}

	handler, e := newMailHandler(map[string]interface{}{}, map[string]interface{}{"to_address": *test_mail_to, "subject": "this is test subject!", "content": `
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
	if "" == *defaultSmtpServer {
		t.Skip("please set 'test.mail_to', 'mail.from' and 'mail.smtp_server'")
		return
	}
	handler, e := newMailHandler(map[string]interface{}{}, map[string]interface{}{"arguments": map[string]interface{}{"client": "mei"},
		"to_address": *test_mail_to, "subject": "this is test subject!", "content": `
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

func TestMailHandlerAttachments(t *testing.T) {
	if "" == *defaultSmtpServer {
		t.Skip("please set 'test.mail_to', 'mail.from' and 'mail.smtp_server'")
		return
	}

	handler, e := newMailHandler(map[string]interface{}{}, map[string]interface{}{"to_address": *test_mail_to, "subject": "this is test subject!", "content": `
		this is a test mail!
		`,
		"attachments": []interface{}{
			map[string]interface{}{"file": ".travis.yml"},
		}})
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
