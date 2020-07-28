package delayed_job

import (
	"bytes"
	"flag"
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"errors"

	"github.com/runner-mei/delayed_job/ns20"
)

var gammu_config string
var gammu string
var gammu_with_smsd = flag.Bool("with_smsd", false, "send sms by smsd")

var smsMethod string
var smsNS20Address string
var smsNS20Port string
var smsNS20Timeout int

func init() {
	if runtime.GOOS == "windows" {
		flag.StringVar(&gammu_config, "gammu_config", "data/conf/gammu.conf", "the path of gaummu")
		flag.StringVar(&gammu, "gammu", "runtime_env/gammu/gammu.exe", "the path of gaummu")
	} else {
		flag.StringVar(&gammu_config, "gammu_config", "/etc/tpt/gammu.conf", "the path of gaummu")
		flag.StringVar(&gammu, "gammu", "runtime_env/gammu/gammu", "the path of gaummu")
	}

	flag.StringVar(&smsMethod, "sms.method", "gammu", "")
	flag.StringVar(&smsNS20Address, "sms.ns20.address", "", "")
	flag.StringVar(&smsNS20Port, "sms.ns20.port", "", "")
	flag.IntVar(&smsNS20Timeout, "sms.ns20.timeout", 0, "")
}

var GetUserPhone func(id string) (string, error)

// SendSMS 发送 sms 的钩子
var SendSMS func(method, phone, content string) error

type smsHandler struct {
	content              string
	phone_numbers        []string
	failed_phone_numbers []string
}

func readPhoneNumbers(params map[string]interface{}) ([]string, error) {
	//users := stringsWithDefault(params, "users", ",", nil)
	phoneNumbers := stringsWithDefault(params, "phone_numbers", ",", nil)
	if 0 == len(phoneNumbers) {
		phoneNumbers = stringsWithDefault(params, "phoneNumbers", ",", nil)
	}

	numbers := make([]string, 0, len(phoneNumbers))
	for _, number := range phoneNumbers {
		lines := SplitLines(number)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				numbers = append(numbers, line)
			}
		}
	}
	phoneNumbers = numbers

	if userIDs := stringsWithDefault(params, "users", ",", nil); len(userIDs) > 0 {
		for _, id := range userIDs {
			if id == "" || id == "0" {
				continue
			}

			if GetUserPhone == nil {
				log.Println("GetUserPhone hook is empty")
				continue
			}
			phone, err := GetUserPhone(id)
			if err != nil {
				return nil, err
			}
			if phone == "" {
				log.Println("phone is missing for user", id)
			} else {
				phoneNumbers = append(phoneNumbers, phone)
				// log.Println("phone is '", phone, "' for user", id)
			}
		}
	}
	return phoneNumbers, nil
}

func newSMSHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	phone_numbers, err := readPhoneNumbers(params)
	if err != nil {
		return nil, err
	}
	if 0 == len(phone_numbers) {
		return nil, errors.New("'phone_numbers' is required")
	}

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
	if smsLimiter != nil {
		if !smsLimiter.CanSend() {
			log.Println("超过限制不能再发了")
			return nil
		}
	}

	var phone_numbers []string
	var last error
	for _, phone := range self.phone_numbers {
		if "" == strings.TrimSpace(phone) || "null" == strings.TrimSpace(phone) {
			continue
		}

		var e error
		if SendSMS != nil {
			e = SendSMS(smsMethod, phone, self.content)
		} else {
			switch smsMethod {
			case "", "gammu":
				e = SendByGammu(phone, self.content)
			case "ns20":
				e = SendByNS20(phone, self.content)
			default:
				e = errors.New("sms method '" + smsMethod + "' is unknown")
			}
		}

		if nil != e {
			phone_numbers = append(phone_numbers, phone)
			last = e
			continue
		}
		smsLimiter.Add(1)
	}
	self.failed_phone_numbers = phone_numbers
	return last
}

func SendByGammu(phone, content string) error {
	var excepted string
	var cmd *exec.Cmd
	if *gammu_with_smsd {
		var gammuPath = filepath.Join(filepath.Dir(gammu), "gammu-smsd-inject")
		if "windows" == runtime.GOOS {
			gammuPath = gammuPath + ".exe"
		}

		//gammu-smsd-inject TEXT 123456 -autolen 130 -unicode -text "All your base are belong to us"
		cmd = exec.Command(gammuPath, "-c", gammu_config, "TEXT", phone, "-autolen", "130", "-unicode", "-text", content)
		excepted = "Written message with ID"
	} else {
		cmd = exec.Command(gammu, "-c", gammu_config, "sendsms", "TEXT", phone, "-autolen", "130", "-unicode", "-text", content)
		excepted = "waiting for network answer..OK"
	}

	timer := time.AfterFunc(10*time.Minute, func() {
		defer recover()
		cmd.Process.Kill()
	})
	output, e := cmd.CombinedOutput()
	timer.Stop()

	if e != nil {
		txt := string(bytes.TrimSpace(output))
		if "" != txt {
			return errors.New(txt)
		}
		return e
	}

	if !bytes.Contains(output, []byte(excepted)) {
		return errors.New(string(output))
	}
	return nil
}

func SendByNS20(phone, content string) error {
	if smsNS20Address == "" {
		return errors.New("sms.ns20.address is empty")
	}
	if smsNS20Port == "" {
		return errors.New("sms.ns20.port is empty")
	}
	timeout := 5 * time.Minute
	if smsNS20Timeout > 0 {
		timeout = time.Duration(smsNS20Timeout) * time.Second
	}

	return ns20.Send(net.JoinHostPort(smsNS20Address, smsNS20Port), phone, content, timeout)
}

func init() {
	Handlers["sms"] = newSMSHandler
	Handlers["sms_action"] = newSMSHandler
	Handlers["sms_command"] = newSMSHandler
}
