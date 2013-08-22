package delayed_job

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSyslogHandlerParameterError(t *testing.T) {
	// _, e := newSyslogHandler(nil,
	// 	map[string]interface{}{"rules": []interface{}{}})
	// if nil == e {
	// 	t.Error("excepted error is not nil, but actual is nil")
	// } else if "ctx is nil" != e.Error() {
	// 	t.Error("excepted error is 'ctx is nil', but actual is", e)
	// }

	_, e := newSyslogHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'content' is required." != e.Error() {
		t.Error("excepted error is ''content' is required', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"to_address": "127.0.0.1"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'content' is required." != e.Error() {
		t.Error("excepted error is ''content' is required', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{"redis": 0},
		map[string]interface{}{"to_address": "e"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'to_address' is empty or invalid") {
		t.Error("excepted error contains ''to_address' is empty or invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"facility": "", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'facility' is required." != e.Error() {
		t.Error("excepted error is ['facility' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"facility": "a", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'facility' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"severity": "", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'severity' is required." != e.Error() {
		t.Error("excepted error is ['severity' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"severity": "a", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'severity' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"hostname": "", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'hostname' is required." != e.Error() {
		t.Error("excepted error is ['hostname' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"hostname": "a a", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'hostname' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"tag": "", "to_address": "127.0.0.1:514"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'tag' is required." != e.Error() {
		t.Error("excepted error is ['tag' is required.], but actual is", e)
	}
}

func syslogTest(t *testing.T, cb func(client net.PacketConn, port string, c chan string)) {
	client, err := net.ListenPacket("udp", ":0")
	if nil != err {
		t.Error(err)
		return
	}
	defer client.Close()

	// var wait sync.WaitGroup
	// wait.Add(1)
	// defer wait.Wait()

	c := make(chan string, 100)
	go func() {
		//defer wait.Done()
		for {
			bs := make([]byte, 1024)
			n, _, e := client.ReadFrom(bs)
			if nil != e {
				break
			}
			c <- string(bs[0:n])
		}
	}()

	laddr := client.LocalAddr().String()
	ar := strings.Split(laddr, ":")

	cb(client, ar[len(ar)-1], c)

	client.Close()
}

func TestSyslogHandler(t *testing.T) {
	now := time.Now()
	now_str := now.Format(time.Stamp)

	for idx, test := range []struct {
		message string
		args    map[string]interface{}
	}{{message: "2334567788", args: map[string]interface{}{"to_address": "127.0.0.1:", "content": "2334567788"}},
		{message: "<14>", args: map[string]interface{}{"to_address": "127.0.0.1:", "content": "2334567788"}},
		{message: "<81>", args: map[string]interface{}{"to_address": "127.0.0.1:", "facility": "authpriv", "severity": "alert", "content": "2334567788"}},
		{message: "tag_teset", args: map[string]interface{}{"to_address": "127.0.0.1:", "tag": "tag_teset", "severity": "alert", "content": "2334567788"}},
		{message: "test_host", args: map[string]interface{}{"to_address": "127.0.0.1:", "hostname": "test_host", "severity": "alert", "content": "2334567788"}},
		{message: now_str, args: map[string]interface{}{"to_address": "127.0.0.1:", "timestamp": now, "severity": "alert", "content": "2334567788"}},
		{message: "a1_test a2_test", args: map[string]interface{}{"to_address": "127.0.0.1:", "content": "{{.a1}} {{.a2}}", "arguments": map[string]interface{}{"a1": "a1_test", "a2": "a2_test"}}},
		{message: "a1_test <no value>", args: map[string]interface{}{"to_address": "127.0.0.1:", "content": "{{.a1}} {{.a3}}", "arguments": map[string]interface{}{"a1": "a1_test", "a2": "a2_test"}}}} {

		syslogTest(t, func(client net.PacketConn, port string, c chan string) {
			test.args["to_address"] = test.args["to_address"].(string) + port

			syslog, e := newSyslogHandler(map[string]interface{}{}, test.args)
			if nil != e {
				t.Error(e)
				return
			}

			e = syslog.Perform()
			if nil != e {
				t.Error(e)
				return
			}

			select {
			case s := <-c:
				if !strings.Contains(s, test.message) {
					t.Error("tests[", idx, "] excepted message contains [", test.message, "], but actual is ", s)
				}
			case <-time.After(1 * time.Second):
				t.Error("recv syslog time out")
			}
		})
	}
}
