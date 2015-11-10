package delayed_job

import (
	"bytes"
	"flag"
	"os/exec"
	"strings"

	"errors"
)

var gammu_config = flag.String("gammu_config", "data/etc/gammu.conf", "the path of gaummu")
var gammu = flag.String("gammu", "runtime_env/gammu/gammu.exe", "the path of gaummu")

type smsHandler struct {
	content              string
	phone_numbers        []string
	failed_phone_numbers []string
}

func newSMSHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	users := stringsWithDefault(params, "users", ",", nil)
	phone_numbers := stringsWithDefault(params, "phone_numbers", ",", nil)

	if 0 == len(phone_numbers) && 0 == len(users) {
		return nil, errors.New("'phone_numbers' is required.")
	}

	if 0 == len(phone_numbers) {
		phone_numbers = users
	} else if 0 != len(users) {
		phone_numbers = append(phone_numbers, users...)
	}

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required.")
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
		content, e = genText(content, args)
		if nil != e {
			return nil, e
		}
	}

	return &smsHandler{content: content, phone_numbers: phone_numbers}, nil
}

func (self *smsHandler) UpdatePayloadObject(options map[string]interface{}) {
	delete(options, "users")
	options["phone_numbers"] = self.failed_phone_numbers
}

func (self *smsHandler) Perform() error {
	var phone_numbers []string
	var last error
	for _, phone := range self.phone_numbers {
		if "" == strings.TrimSpace(phone) || "null" == strings.TrimSpace(phone) {
			continue
		}

		cmd := exec.Command(*gammu, "-c", *gammu_config, "sendsms", "TEXT", phone, "-unicode", "-text", self.content)
		output, e := cmd.CombinedOutput()
		if nil != e {
			phone_numbers = append(phone_numbers, phone)
			last = e
			continue
		}
		if !bytes.Contains(output, []byte("waiting for network answer..OK")) {
			phone_numbers = append(phone_numbers, phone)
			last = errors.New(string(output))
			continue
		}
	}
	self.failed_phone_numbers = phone_numbers
	return last
}

func init() {
	Handlers["sms"] = newSMSHandler
	Handlers["sms_action"] = newSMSHandler
}
