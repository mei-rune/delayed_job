package delayed_job

import (
	"testing"
)

func TestMultiplexedHandlerParameterError(t *testing.T) {
	_, e := newMultiplexedHandler(nil,
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "ctx is nil" != e.Error() {
		t.Error("excepted error is 'ctx is nil', but actual is", e)
	}

	_, e = newMultiplexedHandler(map[string]interface{}{},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "backend in the ctx is required" != e.Error() {
		t.Error("excepted error is 'backend in the ctx is required', but actual is", e)
	}

	_, e = newMultiplexedHandler(map[string]interface{}{"backend": 0},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "backend in the ctx is not a backend - int" != e.Error() {
		t.Error("excepted error is 'backend in the ctx is not a backend - int', but actual is", e)
	}

	var client *dbBackend = nil
	_, e = newMultiplexedHandler(map[string]interface{}{"backend": client},
		map[string]interface{}{"rules": []interface{}{}})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "backend in the ctx is nil" != e.Error() {
		t.Error("excepted error is 'backend in the ctx is nil', but actual is", e)
	}

	_, e = newMultiplexedHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}
}

func TestMultiplexedHandler(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		multiplexed, e := newMultiplexedHandler(map[string]interface{}{"backend": backend},
			map[string]interface{}{"rules": []interface{}{}})
		if nil != e {
			t.Error(e)
			return
		}
		e = multiplexed.Perform()
		if nil != e {
			t.Error(e)
			return
		}

		count := int64(-1)
		e = backend.db.QueryRow("SELECT count(*) FROM " + *table_name).Scan(&count)
		if nil != e {
			t.Error(e)
			return
		}

		if count != 0 {
			t.Error("excepted job is empty, actual is ", count)
			return
		}
	})
}

func TestMultiplexedHandler2(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		multiplexed, e := newMultiplexedHandler(map[string]interface{}{"backend": backend}, map[string]interface{}{"priority": 21, "queue": "cc", "rules": []interface{}{
			map[string]interface{}{"type": "test", "priority": 23}, map[string]interface{}{"type": "test", "queue": "aa"}}})
		if nil != e {
			t.Error(e)
			return
		}
		e = multiplexed.Perform()
		if nil != e {
			t.Error(e)
			return
		}

		assertCount := func(t *testing.T, sql string, excepted int64) {
			count := int64(-1)
			e := backend.db.QueryRow(sql).Scan(&count)
			if nil != e {
				t.Error(e)
				return
			}

			if count != excepted {
				t.Error("excepted \"", sql, "\" is ", excepted, ", actual is ", count)
			}
		}

		assertCount(t, "SELECT count(*) FROM "+*table_name, 2)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where priority = 23", 1)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where queue = 'aa'", 1)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where queue = 'cc'", 1)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where priority = 21", 1)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where priority = 21 and queue = 'aa'", 1)
		assertCount(t, "SELECT count(*) FROM "+*table_name+" where priority = 23 and queue = 'cc'", 1)
	})
}
