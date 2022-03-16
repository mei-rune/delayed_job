package delayed_job

import (
	"flag"
	"os"
	"testing"
)

var (
	ding_webhook = flag.String("ding_webhook", os.Getenv("ding_webhook"), "")
	ding_secret  = flag.String("ding_secret", os.Getenv("ding_secret"), "")
	ding_targets = flag.String("ding_targets", os.Getenv("ding_targets"), "")
)

func TestDingtalkHandler(t *testing.T) {
	if "" == *ding_webhook {
		t.Skip("dingtalk is skipped.")
	}

	handler, e := newDingHandler(nil, map[string]interface{}{
		"type":    "dingtalk",
		"webhook": *ding_webhook,
		"secret":  *ding_secret,
		"targets": *ding_targets,
		"content": "TEST this is test message.",
	})
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
