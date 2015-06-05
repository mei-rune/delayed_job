package delayed_job

import (
	"flag"

	"testing"
)

var (
	corp_id     = flag.String("corp_id", "", "")
	corp_secret = flag.String("corp_secret", "", "")
)

func TestWeixinHandler(t *testing.T) {
	if "" == *corp_id {
		t.Skip("weixin is skipped.")
	}

	handler, e := newWeixinHandler(nil, map[string]interface{}{"type": "weixin",
		"corp_id":     *corp_id,
		"corp_secret": *corp_secret,
		"target_type": "",
		"targets":     "",
		"content":     "this is test message.",
		"agent_id":    "1"})
	if nil != e {
		t.Error(e)
		// if e.Error() != test.excepted_error {
		// 	t.Error(e)
		// }
		return
	}

	e = handler.Perform()
	if nil != e {
		t.Error(e)
		// if e.Error() != test.excepted_error {
		// 	t.Error(e)
		// }
		return
	}
}
