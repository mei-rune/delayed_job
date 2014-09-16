package delayed_job

import (
	"bytes"
	"flag"
	"os/exec"

	"errors"
)

var gammu = flag.String("gammu", "runtime_env/gammu/gammu.exe", "the path of gaummu")

type smsHandler struct {
	content      string
	phone_number string
}

func newSMSHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	phone_number := stringWithDefault(params, "phone_number", "")
	if 0 == len(phone_number) {
		return nil, errors.New("'phone_number' is required.")
	}

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required.")
	}
	return &smsHandler{content: content, phone_number: phone_number}, nil
}

func (self *smsHandler) Perform() error {
	cmd := exec.Command(*gammu, "sendsms", "TEXT", self.phone_number, "-unicode", "-textutf8", self.content)
	output, e := cmd.CombinedOutput()
	if nil != e {
		return e
	}
	if !bytes.Contains(output, []byte("OK")) {
		return errors.New(string(output))
	}
	return nil
}

func init() {
	Handlers["sms"] = newSMSHandler
}
