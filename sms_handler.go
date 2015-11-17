package delayed_job

import (
	"bytes"
	"flag"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"errors"
)

var gammu_config = flag.String("gammu_config", "data/etc/gammu.conf", "the path of gaummu")
var gammu = flag.String("gammu", "runtime_env/gammu/gammu.exe", "the path of gaummu")
var gammu_with_smsd = flag.Bool("with_smsd", false, "send sms by smsd")

type smsHandler struct {
	content              string
	phone_numbers        []string
	failed_phone_numbers []string
}

func newSMSHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	//users := stringsWithDefault(params, "users", ",", nil)
	phone_numbers := stringsWithDefault(params, "phone_numbers", ",", nil)

	if 0 == len(phone_numbers) {
		return nil, errors.New("'phone_numbers' is required.")
	}

	// if 0 == len(phone_numbers) {
	// 	phone_numbers = users
	// } else if 0 != len(users) {
	// 	phone_numbers = append(phone_numbers, users...)
	// }

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

		//gammu-smsd-inject TEXT 123456 -text "All your base are belong to us"

		var excepted string
		var cmd *exec.Cmd
		if *gammu_with_smsd {
			var gammu_path = filepath.Join(filepath.Dir(*gammu), "gammu-smsd-inject")
			if "windows" == runtime.GOOS {
				gammu_path = gammu_path + ".exe"
			}

			cmd = exec.Command(gammu_path, "-c", *gammu_config, "TEXT", phone, "-unicode", "-text", self.content)
			excepted = "Written message with ID"
		} else {
			cmd = exec.Command(*gammu, "-c", *gammu_config, "sendsms", "TEXT", phone, "-unicode", "-text", self.content)
			excepted = "waiting for network answer..OK"
		}

		timer := time.AfterFunc(10*time.Minute, func() {
			defer recover()
			cmd.Process.Kill()
		})
		output, e := cmd.CombinedOutput()
		timer.Stop()

		if nil != e {
			phone_numbers = append(phone_numbers, phone)
			txt := strings.TrimSpace(string(output))
			if "" == txt {
				last = e
				continue
			}

			last = errors.New(txt)
			continue
		}

		if !bytes.Contains(output, []byte(excepted)) {
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
