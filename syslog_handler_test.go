package delayed_job

import (
	"strings"
	"testing"
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
		map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'content' is required." != e.Error() {
		t.Error("excepted error is ''content' is required', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{"redis": 0},
		map[string]interface{}{"to": "e"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'to' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"facility": ""})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'facility' is required." != e.Error() {
		t.Error("excepted error is ['facility' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"facility": "a"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'facility' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"severity": ""})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'severity' is required." != e.Error() {
		t.Error("excepted error is ['severity' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"severity": "a"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'severity' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"hostname": ""})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'hostname' is required." != e.Error() {
		t.Error("excepted error is ['hostname' is required.], but actual is", e)
	}
	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"hostname": "a a"})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if !strings.Contains(e.Error(), "'hostname' is invalid") {
		t.Error("excepted error contains ''to' is invalid', but actual is", e)
	}

	_, e = newSyslogHandler(map[string]interface{}{},
		map[string]interface{}{"tag": ""})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'tag' is required." != e.Error() {
		t.Error("excepted error is ['tag' is required.], but actual is", e)
	}

	// redisTest(t, func(client *redis_keeper, c redis.Conn) {
	// 	_, e := newSyslogHandler(map[string]interface{}{"redis": client},
	// 		map[string]interface{}{"rules": []interface{}{}})
	// 	if nil == e {
	// 		t.Error("excepted error is not nil, but actual is nil")
	// 	} else if "'command' or 'commands' is required" != e.Error() {
	// 		t.Error("excepted error is ''command' or 'commands' is required', but actual is", e)
	// 	}
	// })
}

// func TestRedisHandler(t *testing.T) {
// 	for idx, test := range []struct {
// 		message string
// 		args    map[string]interface{}
// 	}{{message: "", args: {"to": "127.0.0.1:" + port, "message": "2334567788"}},
// 		{message: "", args: {"to": "127.0.0.1:" + port, "message": "2334567788"}}} {

// 		syslogTest(t, func(client *redis_keeper, c redis.Conn) {

// 			checkResult(t, c, "GET", "a1", "1223")
// 			checkResult(t, c, "GET", "a2", "1224")
// 			checkResult(t, c, "GET", "a3", "1225")
// 			checkResult(t, c, "GET", "a4", "1226")
// 			checkResult(t, c, "GET", "a5", "1227")
// 		})
// 	}
// }
