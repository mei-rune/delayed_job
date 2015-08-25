package delayed_job

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebHandler(t *testing.T) {
	for _, test := range []struct {
		method         string
		url            string
		excepted_url   string
		body           interface{}
		head1          string
		head2          string
		excepted_body  string
		excepted_error string
	}{{method: "GET", url: "/aaa/bbb", head1: "head1", head2: "head2"},
		{method: "GEaT", url: "/aaa/bbb", excepted_error: "unsupported http method - GEaT"},
		{method: "PUT", url: "/aaa/bbb", body: "adsdfdsf", excepted_body: "adsdfdsf", head1: "head1", head2: "head2"},
		{method: "PUT", url: "/aaa/bbb/{{.abc1}}/{{.abc2}}", excepted_url: "/aaa/bbb/aaa/bbb", body: "adsdfdsf", excepted_body: "adsdfdsf", head1: "head1", head2: "head2"},
		{method: "PUT", url: "/aaa/bbb/{{.abc1}}/{{.abc2}}", excepted_url: "/aaa/bbb/aaa/bbb", body: "adsdfdsf{{.abc2}}", excepted_body: "adsdfdsfbbb", head1: "head1", head2: "head2"}} {
		func() {
			var method, url, body, head1, head2 string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				method = req.Method
				url = req.URL.Path
				head1 = req.Header.Get("x")
				head2 = req.Header.Get("aaddd")
				if "HEAD" != req.Method && "GET" != req.Method {
					bs, _ := ioutil.ReadAll(req.Body)
					if nil != bs {
						body = string(bs)
					}
				}
				if "PUT" == req.Method {
					w.WriteHeader(http.StatusAccepted)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer srv.Close()

			handler, e := newHandler(nil, map[string]interface{}{"type": "web",
				"method":     test.method,
				"url":        srv.URL + test.url,
				"body":       test.body,
				"head.x":     test.head1,
				"head.aaddd": test.head2,
				"arguments": map[string]interface{}{
					"abc1": "aaa",
					"abc2": "bbb"}})
			if nil != e {
				if e.Error() != test.excepted_error {
					t.Error(e)
				}
				return
			}

			e = handler.Perform()
			if nil != e {
				if e.Error() != test.excepted_error {
					t.Error(e)
				}
				return
			}

			if method != test.method {
				t.Error("exepted method is ", test.method, ", but actual is", method)
			}

			if "" != test.excepted_url {
				if url != test.excepted_url {
					t.Error("exepted url is ", test.excepted_url, ", but actual is", url)
				}
			} else if url != test.url {
				t.Error("exepted url is ", test.url, ", but actual is", url)
			}

			if body != test.excepted_body {
				t.Error("exepted body is ", test.excepted_body, ", but actual is", body)
			}

			if head1 != test.head1 {
				t.Error("exepted head1 is ", test.head1, ", but actual is", head1)
			}
			if head2 != test.head2 {
				t.Error("exepted head2 is ", test.head2, ", but actual is", head2)
			}
		}()
	}
}
