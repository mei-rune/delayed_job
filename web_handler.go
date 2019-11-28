package delayed_job

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/transform"
)

type WebSMS struct {
	Method       string
	Body         interface{}
	SupportBatch bool
}

var WebsmsTypes = map[string]WebSMS{}

const head_prefix = "head."

type webHandler struct {
	method          string
	urlStr          string
	user            string
	password        string
	contentType     string
	body            interface{}
	headers         map[string]interface{}
	responseCode    int
	responseContent string
	args            map[string]interface{}

	failedPhoneNumbers []string
	phoneNumbers       []string
	supportBatch       bool
}

func newWebHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}
	responseCode := intWithDefault(params, "response_code", -1)
	if -1 == responseCode {
		responseCode = intWithDefault(params, "responseCode", -1)
	}

	responseContent := stringWithDefault(params, "response_content", "")
	if "" == responseContent {
		responseContent = stringWithDefault(params, "responseContent", "")
	}

	method := stringWithDefault(params, "method", "")
	urlStr := stringWithDefault(params, "url", "")
	if 0 == len(urlStr) {
		return nil, errors.New("'url' is required.")
	}

	args, ok := params["arguments"]
	if ok {
		args = preprocessArgs(args)
		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}

			if _, ok := props["triggered_at"]; !ok {
				props["triggered_at"] = time.Now()
			}
		}
	} else {
		args = params

		if _, ok := params["triggered_at"]; !ok {
			params["triggered_at"] = time.Now()
		}
	}

	var e error
	urlStr, e = genText(urlStr, args)
	if nil != e {
		return nil, errors.New("failed to merge 'url' with params, " + e.Error())
	}

	var phoneNumbers []string
	var supportBatch bool
	var body interface{}
	var contentType string

	if "GET" != method {
		var ok bool
		contentType = stringWithDefault(params, "contentType", "")
		body, ok = params["body"]
		if !ok {
			values := map[string]interface{}{}
			for key, value := range params {
				if strings.HasPrefix(key, "body[") && strings.HasSuffix(key, "]") {
					key = strings.TrimPrefix(strings.TrimSuffix(key, "]"), "body[")
					values[key] = value
				} else if strings.HasPrefix(key, "body.") {
					key = strings.TrimPrefix(key, "body.")
					values[key] = value
				}
			}
			if len(values) == 0 {
				websms_type := stringWithDefault(params, "websms_type", "")
				if websms_type == "" {
					return nil, errors.New("body is empty")
				}
				var err error
				config := WebsmsTypes[websms_type]
				phoneNumbers, err = readPhoneNumbers(params)
				if err != nil {
					return nil, err
				}
				if 0 == len(phoneNumbers) {
					return nil, errors.New("'phone_numbers' is required")
				}

				body = config.Body
				supportBatch = config.SupportBatch
				if config.Method != "" {
					method = config.Method
				}
			} else {
				body = values
			}
		}
	}

	headers := map[string]interface{}{}
	var all = []map[string]interface{}{params}
	if o, ok := params["attributes"]; ok && o != nil {
		if attributes, ok := o.(map[string]interface{}); ok {
			all = append(all, attributes)
		} else if s, ok := o.(string); ok {
			json.Unmarshal([]byte(s), &attributes)
			if attributes != nil {
				all = append(all, attributes)
			}
		}
	}
	for idx := range all {
		for k, v := range all[idx] {
			if head_prefix == k {
				continue
			}
			if v == nil {
				continue
			}

			if strings.HasPrefix(k, head_prefix) {
				if s, ok := v.(string); ok {
					v, e = genText(s, args)
					if nil != e {
						return nil, errors.New("failed to merge '" + k + "' with params, " + e.Error())
					}
					headers[k[len(head_prefix):]] = v
				} else {
					headers[k[len(head_prefix):]] = v
				}
			}
		}
	}

	headerText := stringWithDefault(params, "header", "")
	if headerText == "" {
		headerText = stringWithDefault(params, "headers", "")
	}
	if headerText != "" {
		headerText, e = genText(headerText, args)
		if nil != e {
			return nil, errors.New("failed to merge 'headers' with params, " + e.Error())
		}
		lines := SplitLines(headerText)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			kvs := strings.SplitN(line, ":", 2)
			if len(kvs) != 2 {
				continue
			}
			k := strings.TrimSpace(kvs[0])
			v := strings.TrimSpace(kvs[1])
			if k == "" || v == "" {
				continue
			}

			headers[k] = v
		}
	}

	if method == "" {
		return nil, errors.New("'method' is required.")
	}
	switch method {
	case "GET", "PUT", "POST", "DELETE", "TRACE", "HEAD", "OPTIONS", "CONNECT", "PATCH":
	default:
		return nil, errors.New("unsupported http method - " + method)
	}

	user := stringWithDefault(params, "username", "")
	if "" == user {
		user = stringWithDefault(params, "user_name", "")
		if "" == user {
			user = stringWithDefault(params, "userName", "")
		}
	}
	password := stringWithDefault(params, "password", "")
	if "" == password {
		password = stringWithDefault(params, "user_password", "")
		if "" == password {
			password = stringWithDefault(params, "userPassword", "")
		}
	}

	return &webHandler{method: method,
		urlStr:          urlStr,
		contentType:     contentType, //stringWithDefault(params, "contentType", ""),
		responseCode:    responseCode,
		responseContent: responseContent,
		user:            user,
		password:        password,
		body:            body,
		headers:         headers,
		args:            params,
		phoneNumbers:    phoneNumbers,
		supportBatch:    supportBatch,
	}, nil
}

func (self *webHandler) logRequest() {
	log.Println("method=", self.method)
	log.Println("url=", self.urlStr)
	log.Println("headers=", self.headers)
	log.Println("password=", self.password)
	log.Println("contentType=", self.contentType)
	log.Println("body=", self.body)
	log.Println("user=", self.user)
	log.Println("phoneNumbers=", self.phoneNumbers)
	log.Println("supportBatch=", self.supportBatch)
}

func (self *webHandler) UpdatePayloadObject(options map[string]interface{}) {
	if self.supportBatch {
		delete(options, "users")
		options["phone_numbers"] = self.phoneNumbers
		options["phoneNumbers"] = self.phoneNumbers
	} else {
		delete(options, "users")
		options["phone_numbers"] = self.failedPhoneNumbers
	}
}

func (self *webHandler) Perform() error {
	if self.supportBatch {
		var body interface{}
		if self.method != "GET" && self.method != "HEAD" {
			if self.body != nil {
				self.args["phone_numbers"] = self.phoneNumbers
				self.args["phoneNumbers"] = self.phoneNumbers
				value, err := genBody("body", self.body, self.args)
				if err != nil {
					return err
				}
				body = value
			}
		}

		return self.perform(body)
	}
	self.failedPhoneNumbers = self.phoneNumbers

	var failed []string

	var lastErr error
	for _, phone := range self.phoneNumbers {

		var body interface{}
		if self.method != "GET" && self.method != "HEAD" {
			if self.body != nil {
				self.args["phone"] = phone
				value, err := genBody("body", self.body, self.args)
				if err != nil {
					return err
				}
				body = value
			}
		}
		err := self.perform(body)
		if err != nil {
			failed = append(failed, phone)
			lastErr = err
		}
	}
	self.failedPhoneNumbers = failed
	return lastErr
}

func (self *webHandler) perform(body interface{}) error {
	var reader io.Reader
	if self.method != "GET" && self.method != "HEAD" {
		if body != nil {
			if self.contentType == "application/x-www-form-urlencoded" {
				queryParams := url.Values{}
				if ok := toUrlEncoded(body, "", queryParams); ok {
					body = queryParams.Encode()
				}
			}

			if s, ok := body.(string); ok {
				reader = bytes.NewBufferString(s)
			} else if s, ok := body.([]byte); ok {
				reader = bytes.NewBuffer(s)
			} else {
				buffer := bytes.NewBuffer(make([]byte, 0, 1024))
				e := json.NewEncoder(buffer).Encode(body)
				if nil != e {
					return e
				}
				reader = buffer
			}
		}
	}

	req, e := http.NewRequest(self.method, self.urlStr, reader)
	if e != nil {
		return e
	}
	if "" != self.user {
		req.URL.User = url.UserPassword(self.user, self.password)
	}
	if self.contentType != "" {
		req.Header.Set("Content-Type", self.contentType)
	}
	if 0 != len(self.headers) {
		for k, v := range self.headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}

	log.Println("execute web:", self.method, self.urlStr)
	resp, e := http.DefaultClient.Do(req)
	if nil != e {
		self.logRequest()
		return e
	}

	// Install closing the request body (if any)
	defer func() {
		if nil != resp.Body {
			resp.Body.Close()
		}
	}()

	var ok bool
	if self.responseCode <= 0 {
		ok = resp.StatusCode == http.StatusOK
		if !ok && ("POST" == self.method ||
			"PUT" == self.method ||
			"PATCH" == self.method ||
			"DELETE" == self.method) {
			ok = resp.StatusCode == http.StatusCreated ||
				resp.StatusCode == http.StatusAccepted ||
				resp.StatusCode == http.StatusNoContent ||
				resp.StatusCode == http.StatusResetContent ||
				resp.StatusCode == http.StatusPartialContent
		}
	} else {
		ok = resp.StatusCode == self.responseCode
	}

	if !ok {
		self.logRequest()

		respBody, err := ioutil.ReadAll(resp.Body)
		if 0 == len(respBody) {
			return fmt.Errorf("failed to read body - %s", err)
		}
		return fmt.Errorf("%v: %v", resp.StatusCode, string(respBody))
	}
	if "" == self.responseContent {
		respBody, _ := ioutil.ReadAll(resp.Body)
		log.Printf("response is %s", respBody)
		self.logRequest()
		return nil
	}

	if resp.ContentLength < 1024*1024 {
		respBody, err := ioutil.ReadAll(resp.Body)
		if 0 == len(respBody) {
			self.logRequest()
			return fmt.Errorf("failed to read body - %s", err)
		}

		if bytes.Contains(respBody, []byte(self.responseContent)) {
			return nil
		}
		self.logRequest()
		return errors.New("'" + self.responseContent + "' isn't exists in the response body:\r\n" + string(respBody))
	}

	matched, e := IsContains(resp.Body, self.responseContent)
	if nil != e {
		self.logRequest()
		return errors.New("failed to read body - " + e.Error())
	}
	if !matched {
		self.logRequest()
		return errors.New("'" + self.responseContent + "' isn't exists in the response body.")
	}
	return nil
}

func maxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}
func IsContains(r io.Reader, excepted string) (bool, error) {
	exceptedBytes := []byte(excepted)
	buffer := make([]byte, 0, maxInt(1024, len(exceptedBytes)+256))
	remainLength := len(exceptedBytes) - 1
	offset := 0
	for {
		n, e := r.Read(buffer[offset:])
		if nil != e {
			if e == io.EOF {
				return false, nil
			}
			return false, e
		}

		if bytes.Contains(buffer[0:n], exceptedBytes) {
			return true, nil
		}

		if n-remainLength >= 0 {
			copy(buffer, buffer[n-remainLength:n])
			offset = remainLength
		}
	}

	return false, nil
}

func init() {
	Handlers["web"] = newWebHandler
	Handlers["websms"] = newWebHandler
	Handlers["websms_command"] = newWebHandler
	Handlers["web_action"] = newWebHandler
	Handlers["web_command"] = newWebHandler
	Handlers["http"] = newWebHandler
	Handlers["http_action"] = newWebHandler
	Handlers["http_command"] = newWebHandler
	Handlers["itsm_command"] = newWebHandler
}

func parseInterval(s string, defValue time.Duration) time.Duration {
	minus := false
	if strings.HasPrefix(s, "-") {
		minus = true
		s = strings.TrimPrefix(s, "-")
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}

	a, err := time.ParseDuration(s)
	if err != nil {
		return defValue
	}

	if minus {
		return -a
	}
	return a
}

var Funcs = template.FuncMap{
	"timeFormat": func(format string, t interface{}) string {
		now := asTimeWithDefault(t, time.Time{})

		switch {
		case strings.HasPrefix(format, "unix"):
			interval := time.Duration(0)
			if len(format) >= 2 {
				interval = parseInterval(strings.TrimSpace(strings.TrimPrefix(format, "unix")), 0)
			}

			return strconv.FormatInt(now.UTC().Add(interval).Unix(), 10)
		case strings.HasPrefix(format, "unix_ms"):
			interval := time.Duration(0)
			if len(format) >= 2 {
				interval = parseInterval(strings.TrimSpace(strings.TrimPrefix(format, "unix_ms")), 0)
			}
			return strconv.FormatInt(now.UTC().Add(interval).UnixNano()/int64(time.Millisecond), 10)
		}
		return now.Format(format)
	},
	"now": func(format ...string) interface{} {
		if len(format) == 0 {
			return time.Now()
		}

		interval := time.Duration(0)
		if len(format) >= 2 {
			interval = parseInterval(format[1], 0)
		}
		switch format[0] {
		case "unix":
			return strconv.FormatInt(time.Now().Add(interval).UTC().Unix(), 10)
		case "unix_ms":
			return strconv.FormatInt(time.Now().Add(interval).UTC().UnixNano()/int64(time.Millisecond), 10)
		}
		return time.Now().Format(format[0])
	},
	"md5": func(s string) string {
		bs := md5.Sum([]byte(s))
		return hex.EncodeToString(bs[:])
	},
	"base64": func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	},
	"concat": func(args ...interface{}) string {
		var buf bytes.Buffer
		for _, v := range args {
			fmt.Fprint(&buf, v)
		}
		return buf.String()
	},
	"toString": func(v interface{}) string {
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	},
	"toInt": func(v interface{}, defaultValue ...int) int {
		if len(defaultValue) > 0 {
			return asIntWithDefault(v, defaultValue[0])
		}
		return asIntWithDefault(v, 0)
	},
	"toInt64": func(v interface{}, defaultValue ...int64) int64 {
		if len(defaultValue) > 0 {
			return asInt64WithDefault(v, defaultValue[0])
		}
		return asInt64WithDefault(v, 0)
	},
	"keyExists": func(v map[string]interface{}, key string) bool {
		_, ok := v[key]
		return ok
	},
	"keyExist": func(v map[string]interface{}, key string) bool {
		_, ok := v[key]
		return ok
	},
	"toLower": strings.ToLower,
	"toUpper": strings.ToUpper,
	"toTitle": strings.ToTitle,
	"replace": func(old_s, new_s, content string) string {
		return strings.Replace(content, old_s, new_s, -1)
	},
	"queryEscape": QueryEscape,
	"charset_encode": func(charset, content string) string {
		encoding := GetCharset(charset)
		newContent, _, err := transform.String(encoding.NewEncoder(), content)
		if err != nil {
			return content
		}
		return newContent
	},
}

func genText(content string, args interface{}) (string, error) {
	if nil == args {
		return content, nil
	}
	if pos := strings.Index(content, "{{"); pos >= 0 {
		if !strings.Contains(content[pos+2:], "}}") {
			return content, nil
		}
	}
	t, e := template.New("default").Funcs(Funcs).Parse(content)
	if nil != e {
		return content, errors.New("create template failed, " + e.Error())
	}

	switch m := args.(type) {
	case map[string]interface{}:
		if _, ok := m["content"]; !ok {
			m["content"] = "this_is_test_message"
		}
	case map[string]string:
		if _, ok := m["content"]; !ok {
			m["content"] = "this_is_test_message"
		}
	}

	args = preprocessArgs(args)
	var buffer bytes.Buffer
	e = t.Execute(&buffer, args)
	if nil != e {
		return content, errors.New("execute template failed, " + e.Error())
	}
	return buffer.String(), nil
}

func QueryEscape(charset, content string) string {
	encoding := GetCharset(charset)
	newContent, _, err := transform.String(encoding.NewEncoder(), content)
	if err != nil {
		return content
	}
	return url.QueryEscape(newContent)
}

func genBody(prefix string, body, args interface{}) (interface{}, error) {
	switch m := body.(type) {
	case string:
		a, e := genText(m, args)
		if nil != e {
			return nil, errors.New("failed to merge '" + prefix + "' with params, " + e.Error())
		}
		return a, nil
	case map[string]string:
		for key, value := range m {
			a, e := genText(value, args)
			if nil != e {
				return nil, errors.New("failed to merge '" + prefix + "." + key + "' with params, " + e.Error())
			}
			m[key] = a
		}
		return body, nil
	case map[string]interface{}:
		for key, value := range m {
			a, e := genBody(prefix+"."+key, value, args)
			if nil != e {
				return nil, e
			}
			m[key] = a
		}
	}
	return body, nil
}

func toUrlEncoded(body interface{}, prefix string, queryParams url.Values) bool {
	switch m := body.(type) {
	case map[string]string:
		for key, value := range m {
			if prefix != "" {
				key = prefix + "[" + key + "]"
			}
			queryParams.Set(key, value)
		}
		return true
	case map[string]interface{}:
		for key, value := range m {
			if prefix != "" {
				key = prefix + "[" + key + "]"
			}
			if ok := toUrlEncoded(value, key, queryParams); ok {
				continue
			}
			queryParams.Set(key, fmt.Sprint(value))
		}
		return true
	default:
		return false
	}
}
