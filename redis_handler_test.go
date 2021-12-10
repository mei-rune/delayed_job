package delayed_job

import (
	"net"
	"strings"
	"testing"
	
	"github.com/gomodule/redigo/redis"
)

func TestRedisHandlerParameterError(t *testing.T) {
	_, e := newRedisHandler(nil,
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "ctx is nil" != e.Error() {
		t.Error("excepted error is 'ctx is nil', but actual is", e)
	}

	_, e = newRedisHandler(map[string]interface{}{},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'redis' in the ctx is required" != e.Error() {
		t.Error("excepted error is ''redis' in the ctx is required', but actual is", e)
	}

	_, e = newRedisHandler(map[string]interface{}{"redis": 0},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'redis' in the ctx is not a *Redis - int" != e.Error() {
		t.Error("excepted error is 'redis in the ctx is not a backend - int', but actual is", e)
	}

	_, e = newRedisHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}

	var client *redis_gateway = nil
	_, e = newRedisHandler(map[string]interface{}{"redis": client},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'redis' in the ctx is nil" != e.Error() {
		t.Error("excepted error is ''redis' in the ctx is nil', but actual is", e)
	}

	redisTest(t, func(client *redis_gateway, c redis.Conn) {
		_, e := newRedisHandler(map[string]interface{}{"redis": client},
			map[string]interface{}{"rules": []interface{}{}})
		if nil == e {
			t.Error("excepted error is not nil, but actual is nil")
		} else if "'command' or 'commands' is required" != e.Error() {
			t.Error("excepted error is ''command' or 'commands' is required', but actual is", e)
		}
	})
}

// redis_client.c <- &redis_request{commands: [][]string{{"SET", "a1", "1223"}}}
// redis_client.c <- &redis_request{commands: [][]string{{"SET", "a2", "1224"}}}
// redis_client.Send([][]string{{"SET", "a3", "1225"}})
// redis_client.Send([][]string{{"SET", "a4", "1226"}})
// redis_client.Send([][]string{{"SET", "a6", "1227"}})

func TestRedisHandler(t *testing.T) {
	for idx, test := range [][]map[string]interface{}{
		[]map[string]interface{}{{"command": "SET", "arg0": "a1", "arg1": "1223"},
			{"command": "SET", "arg0": "a2", "arg1": "1224"},
			{"command": "SET", "arg0": "a3", "arg1": "1225"},
			{"command": "SET", "arg0": "a4", "arg1": "1226"},
			{"command": "SET", "arg0": "a5", "arg1": "1227"}},

		[]map[string]interface{}{{"commands": []interface{}{map[string]interface{}{"command": "SET", "arg0": "a1", "arg1": "1223"},
			map[string]interface{}{"command": "SET", "arg0": "a2", "arg1": "1224"},
			map[string]interface{}{"command": "SET", "arg0": "a3", "arg1": "1225"},
			map[string]interface{}{"command": "SET", "arg0": "a4", "arg1": "1226"},
			map[string]interface{}{"command": "SET", "arg0": "a5", "arg1": "1227"}}}},

		[]map[string]interface{}{{"commands": []interface{}{[]interface{}{"SET", "a1", "1223"},
			[]interface{}{"SET", "a2", "1224"},
			[]interface{}{"SET", "a3", "1225"},
			[]interface{}{"SET", "a4", "1226"},
			[]interface{}{"SET", "a5", "1227"}}}},

		[]map[string]interface{}{{"commands": []interface{}{[]string{"SET", "a1", "1223"},
			[]string{"SET", "a2", "1224"},
			[]string{"SET", "a3", "1225"},
			[]string{"SET", "a4", "1226"},
			[]string{"SET", "a5", "1227"}}}},

		[]map[string]interface{}{{"arguments": map[string]interface{}{"a1": "1223", "a2": "1224"},
			"commands": []interface{}{[]string{"SET", "a1", "$a1"},
				[]string{"SET", "a2", "$a2"},
				[]string{"SET", "a3", "1225"},
				[]string{"SET", "a4", "1226"},
				[]string{"SET", "a5", "1227"}}}},

		[]map[string]interface{}{{"arguments": map[string]string{"a1": "1223", "a2": "1224"},
			"commands": []interface{}{[]string{"SET", "a1", "$a1"},
				[]string{"SET", "a2", "$a2"},
				[]string{"SET", "a3", "1225"},
				[]string{"SET", "a4", "1226"},
				[]string{"SET", "a5", "1227"}}}},

		[]map[string]interface{}{{"arguments": []interface{}{map[string]string{"a1": "1223", "a2": "1224"}},
			"commands": []interface{}{[]string{"SET", "a1", "$[0].a1"},
				[]string{"SET", "a2", "$[0].a2"},
				[]string{"SET", "a3", "1225"},
				[]string{"SET", "a4", "1226"},
				[]string{"SET", "a5", "1227"}}}},

		[]map[string]interface{}{{"arguments": []interface{}{"1223", map[string]interface{}{"a1": "1223", "a2": "1224"}},
			"commands": []interface{}{[]string{"SET", "a1", "$[0]"},
				[]string{"SET", "a2", "$[1].a2"},
				[]string{"SET", "a3", "1225"},
				[]string{"SET", "a4", "1226"},
				[]string{"SET", "a5", "1227"}}}}} {

		redisTest(t, func(client *redis_gateway, c redis.Conn) {
			for _, args := range test {
				handler, e := newRedisHandler(map[string]interface{}{"redis": client}, args)
				if nil != e {
					t.Error("execute [", idx, "]", e)
					return
				}

				e = handler.Perform()
				if nil != e {
					t.Error("execute [", idx, "]", e)
					return
				}
			}

			checkResult(t, c, "GET", "a1", "1223")
			checkResult(t, c, "GET", "a2", "1224")
			checkResult(t, c, "GET", "a3", "1225")
			checkResult(t, c, "GET", "a4", "1226")
			checkResult(t, c, "GET", "a5", "1227")
		})
	}
}

func TestRedisHandlerFailed(t *testing.T) {

	listener, e := net.Listen("tcp", ":0")
	if nil != e {
		t.Error(e)
		return
	}

	old := *redisAddress

	ss := strings.Split(listener.Addr().String(), ":")

	*redisAddress = "127.0.0.1:" + ss[len(ss)-1]

	defer func() {
		*redisAddress = old
		listener.Close()
	}()

	go func() {
		for {
			t, e := listener.Accept()
			if nil != e {
				return
			}

			t.Close()
		}
	}()

	for _, test := range []map[string]interface{}{
		{"command": "SET", "arg0": "a2", "arg1": "1224"},

		{"commands": []interface{}{[]interface{}{"SET", "a1", "1223"},
			[]interface{}{"SET", "a2", "1224"},
			[]interface{}{"SET", "a3", "1225"},
			[]interface{}{"SET", "a4", "1226"},
			[]interface{}{"SET", "a5", "1227"}}}} {

		redisTest(t, func(client *redis_gateway, c redis.Conn) {
			handler, e := newRedisHandler(map[string]interface{}{"redis": client}, test)
			if nil != e {
				t.Error(e)
				return
			}

			e = handler.Perform()
			if nil == e {
				t.Error("excepted error is not nil, but actual is nil")
				return
			}
		})
	}
}
