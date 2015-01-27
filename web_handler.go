package delayed_job

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"text/template"
)

const head_prefix = "head."

type webHandler struct {
	method   string
	url      string
	user     string
	password string
	body     interface{}
	headers  map[string]interface{}
}

func newWebHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
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
	url, e := genText(url, params)
	if nil != e {
		return nil, errors.New("failed to merge 'url' with params, " + e.Error())
	}
	if s, ok := body.(string); ok {
		body, e = genText(s, params)
		if nil != e {
			return nil, errors.New("failed to merge 'body' with params, " + e.Error())
		}
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
	return &webHandler{method: method,
		url:      url,
		user:     stringWithDefault(params, "user_name", ""),
		password: stringWithDefault(params, "user_password", ""),
		body:     body, headers: headers}, nil
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

	if resp.StatusCode != 200 && ("POST" != self.method || resp.StatusCode != 201) {
		resp_body, _ := ioutil.ReadAll(resp.Body)
		if nil == resp_body || 0 == len(resp_body) {
			return fmt.Errorf("%v: error", resp.StatusCode)
		}
		return fmt.Errorf("%v: %v", resp.StatusCode, string(resp_body))
	}

	return nil
}

func init() {
	Handlers["web"] = newWebHandler
}

func genText(content string, args interface{}) (string, error) {
	if nil == args {
		return content, nil
	}
	if !strings.Contains(content, "{{") {
		return content, nil
	}
	t, e := template.New("default").Parse(content)
	if nil != e {
		return content, errors.New("create template failed, " + e.Error())
	}
	var buffer bytes.Buffer
	e = t.Execute(&buffer, args)
	if nil != e {
		return content, errors.New("execute template failed, " + e.Error())
	}
	return buffer.String(), nil
}
