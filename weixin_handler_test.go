package delayed_job

import (
	"flag"

	"testing"
)

var (
	weixin_corp_id     = flag.String("weixin_corp_id", "", "")
	weixin_corp_secret = flag.String("weixin_corp_secret", "", "")
	weixin_target_type = flag.String("weixin_target_type", "user", "")
	weixin_targets     = flag.String("weixin_targets", "", "")
	weixin_agent_id    = flag.String("weixin_agent_id", "1", "")
)

func TestWeixinHandler(t *testing.T) {
	if "" == *weixin_corp_id {
		t.Skip("weixin is skipped.")
	}

	handler, e := newWeixinHandler(nil, map[string]interface{}{"type": "weixin",
		"corp_id":     *weixin_corp_id,
		"corp_secret": *weixin_corp_secret,
		"target_type": *weixin_target_type,
		"targets":     *weixin_targets,
		"content":     "this is test message.",
		"agent_id":    *weixin_agent_id})
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
