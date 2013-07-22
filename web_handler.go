package delayed_job

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const head_prefix = "head."

type webHandler struct {
	method string
	url    string
	body   interface{}
	header map[string]interface{}
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

	url := stringWithDefault(params, "url", "")
	if 0 == len(url) {
		return nil, errors.New("'url' is required.")
	}
	body := params["arguments"]

	header := map[string]interface{}{}
	for k, v := range params {
		if head_prefix == k {
			continue
		}

		if strings.HasPrefix(k, head_prefix) {
			header[k[len(head_prefix):]] = v
		}
	}
	return &webHandler{method: method, url: url, body: body, header: header}, nil
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

	if 0 != len(self.header) {
		for k, v := range self.header {
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