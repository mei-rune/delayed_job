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

var gammu_config string
var gammu string
var gammu_with_smsd = flag.Bool("with_smsd", false, "send sms by smsd")

func init() {
	if runtime.GOOS == "windows" {
		flag.StringVar(&gammu_config, "gammu_config", "data/conf/gammu.conf", "the path of gaummu")
		flag.StringVar(&gammu, "gammu", "runtime_env/gammu/gammu.exe", "the path of gaummu")
	} else {
		flag.StringVar(&gammu_config, "gammu_config", "/etc/tpt/gammu.conf", "the path of gaummu")
		flag.StringVar(&gammu, "gammu", "runtime_env/gammu/gammu", "the path of gaummu")
	}
}

// SendSMS 发送 sms 的钩子
var SendSMS func(phone, content string) error

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
		phone_numbers = stringsWithDefault(params, "phoneNumbers", ",", nil)
		if 0 == len(phone_numbers) {
			return nil, errors.New("'phone_numbers' is required")
		}
	}

	numbers := make([]string, 0, len(phone_numbers))
	for _, number := range phone_numbers {
		lines := SplitLines(number)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				numbers = append(numbers, line)
			}
		}
	}
	phone_numbers = numbers
	if 0 == len(phone_numbers) {
		return nil, errors.New("'phone_numbers' is required")
	}

	// if 0 == len(phone_numbers) {
	// 	phone_numbers = users
	// } else if 0 != len(users) {
	// 	phone_numbers = append(phone_numbers, users...)
	// }

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required")
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

		var e error
		if SendSMS != nil {
			e = SendSMS(phone, self.content)
		} else {
			var excepted string
			var output []byte
			var cmd *exec.Cmd
			if *gammu_with_smsd {
				var gammuPath = filepath.Join(filepath.Dir(gammu), "gammu-smsd-inject")
				if "windows" == runtime.GOOS {
					gammuPath = gammuPath + ".exe"
				}

				//gammu-smsd-inject TEXT 123456 -text "All your base are belong to us"
				cmd = exec.Command(gammuPath, "-c", gammu_config, "TEXT", phone, "-unicode", "-text", self.content)
				excepted = "Written message with ID"
			} else {
				cmd = exec.Command(gammu, "-c", gammu_config, "sendsms", "TEXT", phone, "-unicode", "-text", self.content)
				excepted = "waiting for network answer..OK"
			}

			timer := time.AfterFunc(10*time.Minute, func() {
				defer recover()
				cmd.Process.Kill()
			})
			output, e = cmd.CombinedOutput()
			timer.Stop()

			if e == nil {
				if !bytes.Contains(output, []byte(excepted)) {
					e = errors.New(string(output))
				}
			} else {
				txt := string(bytes.TrimSpace(output))
				if "" != txt {
					e = errors.New(txt)
				}
			}
		}

		if nil != e {
			phone_numbers = append(phone_numbers, phone)
			last = e
			continue
		}
	}
	self.failed_phone_numbers = phone_numbers
	return last
}

func init() {
	Handlers["sms"] = newSMSHandler
	Handlers["sms_action"] = newSMSHandler
	Handlers["sms_command"] = newSMSHandler
}
