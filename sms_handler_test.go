package delayed_job

import (
	"flag"

	"testing"
)

var (
	phone_number = flag.String("phone_number", "", "the phone number")
	sms_content  = flag.String("sms_content", "恒维tpt软件", "the message")
)

func TestSMSHandler(t *testing.T) {
	handler, e := newHandler(nil, map[string]interface{}{"type": "sms",
		"phone_number": *phone_number,
		"content":      *sms_content})
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
