package delayed_job

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	alyunsms "github.com/aliyun-sdk/sms-go"
	"github.com/runner-mei/delayed_job/ns20"
	"github.com/runner-mei/delayed_job/muboat"
	"github.com/runner-mei/delayed_job/j311"
)

var smsLogger *log.Logger
var gammu_config string
var gammu string
var gammu_with_smsd = flag.Bool("with_smsd", false, "send sms by smsd")

var smsMethod string
var smsNS20Address string
var smsNS20Port string
var smsNS20Timeout int

var smsMuboatV1Address string
var smsMuboatV1Port string
var smsMuboatV1Timeout int


var smsj311Address string
var smsj311Port string
var smsj311Timeout int
var smsj311Charset string

var smsf405Address string
var smsf405Port string
var smsf405Timeout int
var smsf405Charset string

var GetAliyunClientParams func() (accessKey, secretKey, signName, templateCode string)

func SetSMSLogger(logger *log.Logger) {
	smsLogger = logger
}

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

	// flag.StringVar(&smsMethod, "sms.method", "muboat_v1", "")
	flag.StringVar(&smsMuboatV1Address, "sms.muboat_v1.address", "", "")
	flag.StringVar(&smsMuboatV1Port, "sms.muboat_v1.port", "", "")
	flag.IntVar(&smsMuboatV1Timeout, "sms.muboat_v1.timeout", 0, "")


	flag.StringVar(&smsj311Address, "sms.j311.address", "", "")
	flag.StringVar(&smsj311Port, "sms.j311.port", "", "")
	flag.IntVar(&smsj311Timeout, "sms.j311.timeout", 0, "")
	flag.StringVar(&smsj311Charset, "sms.j311.charset", "", "")

	flag.StringVar(&smsf405Address, "sms.f405.address", "", "")
	flag.StringVar(&smsf405Port, "sms.f405.port", "", "")
	flag.IntVar(&smsf405Timeout, "sms.f405.timeout", 0, "")
	flag.StringVar(&smsf405Charset, "sms.f405.charset", "", "")
}

var GetUserPhone func(id string) (string, error)

// SendSMS 发送 sms 的钩子
var SendSMS func(method, phone, content string) error

type smsHandler struct {
	method               string
	args                 interface{}
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

	// fmt.Println("====== ctx =", ctx)
	// fmt.Println("====== params =", params)

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required")
	}

	args, ok := params["arguments"]
	if ok {
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
	} else {
		args = params

		var e error
		content, e = genText(content, map[string]interface{}{
			"content":      "this is test messge.这是一个测试消息。",
			"triggered_at": time.Now().Format(time.RFC3339),
		})
		if nil != e {
			return nil, e
		}
	}

	method := stringWithDefault(params, "sms.method", "")
	if method == "" {
		method = smsMethod
	}

	return &smsHandler{
		content:       content,
		phone_numbers: phone_numbers,
		method:        method,
		args:          args,
	}, nil
}

func (self *smsHandler) UpdatePayloadObject(options map[string]interface{}) {
	delete(options, "users")
	options["phone_numbers"] = self.failed_phone_numbers
}

func readStringWith(o interface{}, key, defaultValue string) string {
	if o == nil {
		return defaultValue
	}

	args, _ := o.(map[string]interface{})
	if args == nil {
		return defaultValue
	}

	v := args[key]
	if v == nil {
		return defaultValue
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func (self *smsHandler) Perform() error {
	if smsLimiter != nil {
		if !smsLimiter.CanSend() {
			log.Println("超过限制不能再发了")

			if smsLogger != nil {
				smsLogger.Println("[sms] 超过限制不能再发了", self.phone_numbers)
			}
			return nil
		}
	}

	switch self.method {
	case "web":
		support := readStringWith(self.args, "sms.web.batch_support", smsWebBatchSupport)
		if support == "true" ||
			support == "True" ||
			support == "TRUE" {
			return BatchSendByWebSvc(self.args, self.phone_numbers, self.content)
		}
	case "exec":
		support := readStringWith(self.args, "sms.exec.batch_support", smsExecBatchSupport)
		if support == "true" ||
			support == "True" ||
			support == "TRUE" {
			return BatchSendByExec(self.args, self.phone_numbers, self.content)
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
			e = SendSMS(self.method, phone, self.content)
		} else {
			switch self.method {
			case "", "gammu":
				e = SendByGammu(phone, self.content)
			case "ns20":
				e = SendByNS20(phone, self.content)
			case "muboat_v1":
				e = SendByMuboatv1(phone, self.content)
			case "j311":
				e = SendByJ311(phone, self.content)
			case "f405":
				e = SendByF405(phone, self.content)
			case "aliyun":
				e = SendByAliyun(phone, self.content)
			case "web":
				e = SendByWebSvc(self.args, phone, self.content)
			case "exec":
				e = SendByExec(self.args, phone, self.content)
			default:
				e = errors.New("sms method '" + smsMethod + "' is unknown")
			}
		}

		if nil != e {
			phone_numbers = append(phone_numbers, phone)
			last = e
			continue
		}

		if smsLogger != nil {
			smsLogger.Println("[sms]", "["+self.method+"]", phone, self.content)
		}
		if smsLimiter != nil {
			smsLimiter.Add(1)
		}
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

	fmt.Println("[sms]", phone, content, string(output))
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

func SendByMuboatv1(phone, content string) error {
	if smsMuboatV1Address == "" {
		return errors.New("缺省 'sms.muboat_v1.address' 参数")
	}
	if smsMuboatV1Port == "" {
		smsMuboatV1Port = "6006"
	}
	if smsMuboatV1Timeout <= 0 {
		smsMuboatV1Timeout = 30
	}

	conn, err := muboat.Connect(net.JoinHostPort(smsMuboatV1Address, smsMuboatV1Port))
	if err != nil {
		return errors.New("连接短信猫失败, "+err.Error())
	}
	defer conn.Close()

	log.Println("sms connect ok")

	return muboat.SendMessage(conn, false, true, true, time.Duration(smsMuboatV1Timeout) * time.Second, phone, content)
}

func SendByJ311(phone, content string) error {
	if smsj311Address == "" {
		return errors.New("缺省 'sms.j311.address' 参数")
	}
	if smsj311Port == "" {
		smsj311Port = "8234"
	}
	if smsj311Timeout <= 0 {
		smsj311Timeout = 30
	}

	return j311.SendMessage(net.JoinHostPort(smsj311Address, smsj311Port),
			time.Duration(smsj311Timeout)*time.Second,
			j311.SMS,	smsj311Charset, phone, content)
}

func SendByF405(phone, content string) error {
	// if smsf405Address == "" {
	// 	return errors.New("缺省 'sms.f405.address' 参数")
	// }
	// if smsf405Port == "" {
	// 	smsf405Port = "8234"
	// }
	// if smsf405Timeout <= 0 {
	// 	smsf405Timeout = 30
	// }

	// return j311.SendMessage(net.JoinHostPort(smsf405Address, smsf405Port),
	// 		time.Duration(smsf405Timeout)*time.Second,
	// 		smsf405Charset, phone, content)
	return errors.New("不支持")
}

var (
	aliyunClientLock sync.Mutex
	aliyunClient     *alyunsms.Client
)

func GetAliyunClient() (*alyunsms.Client, error) {
	aliyunClientLock.Lock()
	defer aliyunClientLock.Unlock()
	if aliyunClient != nil {
		return aliyunClient, nil
	}

	if GetAliyunClientParams == nil {
		return nil, errors.New("GetAliyunClientParams is null")
	}

	accessKey, secretKey, signName, templateCode := GetAliyunClientParams()
	if accessKey == "" {
		return nil, errors.New("请配置好阿里云网关")
	}

	client, err := alyunsms.New(accessKey, secretKey, alyunsms.SignName(signName), alyunsms.Template(templateCode))
	if err != nil {
		aliyunClient = nil
	} else {
		aliyunClient = client
	}

	return client, err
}

func SendByAliyun(phone, content string) error {
	client, err := GetAliyunClient()
	if err != nil {
		return err
	}
	err = client.Send(
		alyunsms.Mobile(phone),
		alyunsms.Parameter(map[string]string{
			"content": content,
		}),
	)
	if err == nil {
		return nil
	}

	aliyunClientLock.Lock()
	defer aliyunClientLock.Unlock()

	// client.Close()
	aliyunClient = nil
	return err
}

func init() {
	Handlers["sms"] = newSMSHandler
	Handlers["sms_action"] = newSMSHandler
	Handlers["sms_command"] = newSMSHandler
}
