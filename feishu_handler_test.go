package delayed_job

import (
	"flag"
	"os"
	"testing"
)

var (
	feishu_webhook = flag.String("feishu_webhook", os.Getenv("feishu_webhook"), "")
	feishu_secret  = flag.String("feishu_secret", os.Getenv("feishu_secret"), "")
	feishu_targets = flag.String("feishu_targets", os.Getenv("feishu_targets"), "")
)

func TestFeishuHandler(t *testing.T) {
	if "" == *feishu_webhook {
		t.Skip("feishu is skipped.")
	}

	handler, e := newFeishuHandler(nil, map[string]interface{}{
		"type":    "feishu",
		"webhook": *feishu_webhook,
		"secret":  *feishu_secret,
		"targets": *feishu_targets,
		"content": "TEST this is test message.",
	})
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
