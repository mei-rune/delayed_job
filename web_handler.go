package delayed_job

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"golang.org/x/text/transform"
)

const head_prefix = "head."

type webHandler struct {
	method          string
	url             string
	user            string
	password        string
	body            interface{}
	headers         map[string]interface{}
	responseCode    int
	responseContent string
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
	if 0 == len(method) {
		return nil, errors.New("'method' is required.")
	}

	switch method {
	case "GET", "PUT", "POST", "DELETE", "TRACE", "HEAD", "OPTIONS", "CONNECT", "PATCH":
	default:
		return nil, errors.New("unsupported http method - " + method)
	}

	body := params["body"]
	url := stringWithDefault(params, "url", "")
	if 0 == len(url) {
		return nil, errors.New("'url' is required.")
	}

	headers := map[string]interface{}{}
	for k, v := range params {
		if head_prefix == k {
			continue
		}

		if strings.HasPrefix(k, head_prefix) {
			headers[k[len(head_prefix):]] = v
		}
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
	} else {
		args = params
	}

	var e error
	url, e = genText(url, args)
	if nil != e {
		return nil, errors.New("failed to merge 'url' with params, " + e.Error())
	}
	if s, ok := body.(string); ok {
		body, e = genText(s, args)
		if nil != e {
			return nil, errors.New("failed to merge 'body' with params, " + e.Error())
		}
	}

	user := stringWithDefault(params, "user_name", "")
	if "" == user {
		user = stringWithDefault(params, "userName", "")
	}
	password := stringWithDefault(params, "user_password", "")
	if "" == password {
		password = stringWithDefault(params, "userPassword", "")
	}

	return &webHandler{method: method,
		url:             url,
		responseCode:    responseCode,
		responseContent: responseContent,
		user:            user,
		password:        password,
		body:            body, headers: headers}, nil
}

func (self *webHandler) Perform() error {
	var reader io.Reader = nil
	switch self.method {
	case "GET", "HEAD":
	default:
		if nil == self.body {
		} else if s, ok := self.body.(string); ok {
			reader = bytes.NewBufferString(s)
		} else {
			buffer := bytes.NewBuffer(make([]byte, 0, 1024))
			e := json.NewEncoder(buffer).Encode(self.body)
			if nil != e {
				return e
			}

			reader = buffer
		}
	}

	req, e := http.NewRequest(self.method, self.url, reader)
	if e != nil {
		return e
	}

	if "" != self.user {
		req.URL.User = url.UserPassword(self.user, self.password)
	}
	if 0 != len(self.headers) {
		for k, v := range self.headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}

	log.Println("execute web:", self.method, self.url)
	resp, e := http.DefaultClient.Do(req)
	if nil != e {
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
		ok = resp.StatusCode == 200
		if !ok && ("POST" == self.method ||
			"PUT" == self.method ||
			"PATCH" == self.method ||
			"DELETE" == self.method) {
			ok = resp.StatusCode == 201 ||
				resp.StatusCode == 202 ||
				resp.StatusCode == 204 ||
				resp.StatusCode == 205 ||
				resp.StatusCode == 206
		}
	} else {
		ok = resp.StatusCode == self.responseCode
	}

	if !ok {
		resp_body, _ := ioutil.ReadAll(resp.Body)
		if nil == resp_body || 0 == len(resp_body) {
			return fmt.Errorf("%v: error", resp.StatusCode)
		}
		return fmt.Errorf("%v: %v", resp.StatusCode, string(resp_body))
	}
	if "" == self.responseContent {
		return nil
	}

	if resp.ContentLength < 1024*1024 {
		resp_body, err := ioutil.ReadAll(resp.Body)
		if nil == resp_body || 0 == len(resp_body) {
			return errors.New("failed to read body - " + err.Error())
		}

		if bytes.Contains(resp_body, []byte(self.responseContent)) {
			return nil
		}
		return errors.New("'" + self.responseContent + "' isn't exists in the response body:\r\n" + string(resp_body))
	}

	matched, e := IsContains(resp.Body, self.responseContent)
	if nil != e {
		return errors.New("failed to read body - " + e.Error())
	}
	if !matched {
		return errors.New("'" + self.responseContent + "' isn't exists in the response body.")
	}
	return nil
}

func max_int(a, b int) int {
	if a < b {
		return b
	}
	return a
}
func IsContains(r io.Reader, excepted string) (bool, error) {
	excepted_bytes := []byte(excepted)
	buffer := make([]byte, 0, max_int(1024, len(excepted_bytes)+256))
	remain_length := len(excepted_bytes) - 1
	offset := 0
	for {
		n, e := r.Read(buffer[offset:])
		if nil != e {
			if e == io.EOF {
				return false, nil
			}
			return false, e
		}

		if bytes.Contains(buffer[0:n], excepted_bytes) {
			return true, nil
		}

		if n-remain_length >= 0 {
			copy(buffer, buffer[n-remain_length:n])
			offset = remain_length
		}
	}

	return false, nil
}

func init() {
	Handlers["web"] = newWebHandler
	Handlers["web_action"] = newWebHandler
	Handlers["web_command"] = newWebHandler
	Handlers["http"] = newWebHandler
	Handlers["http_action"] = newWebHandler
	Handlers["http_command"] = newWebHandler
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
	t, e := template.New("default").Funcs(template.FuncMap{
		"toString": func(v interface{}) string {
			return fmt.Sprint(v)
		},
		"toLower": strings.ToLower,
		"toUpper": strings.ToUpper,
		"toTitle": strings.ToTitle,
		"replace": func(old_s, new_s, content string) string {
			return strings.Replace(content, old_s, new_s, -1)
		},
		"queryEscape": QueryEscape}).Parse(content)
	if nil != e {
		return content, errors.New("create template failed, " + e.Error())
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
	new_content, _, err := transform.String(encoding.NewEncoder(), content)
	if err != nil {
		return content
	}
	return url.QueryEscape(new_content)
}
