package delayed_job

import (
	"database/sql"
	"strings"
	"testing"
)

func TestDbHandlerParameterIsError(t *testing.T) {
	_, e := newDbHandler(nil, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "ctx is nil" != e.Error() {
		t.Error("excepted error is 'ctx is nil', but actual is", e)
	}

	_, e = newDbHandler(map[string]interface{}{}, nil)
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "params is nil" != e.Error() {
		t.Error("excepted error is 'params is nil', but actual is", e)
	}

	_, e = newDbHandler(map[string]interface{}{}, map[string]interface{}{})
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
	} else if "'script' is required." != e.Error() {
		t.Error("excepted error is ['script' is required.], but actual is", e)
	}
}

func TestDbHandlerConnectError(t *testing.T) {
	handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "a", "drv": "postgres", "url": "host=127.0.0.1 port=2345 dbname=tpt_data user=tpt password=extreme sslmode=disable"})
	if nil != e {
		t.Error(e)
		return
	}
	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "dial tcp") {
		t.Error("excepted error contains [dial tcp], but actual is", e)
	}
}

func TestDbHandlerConnectOkAndDbError(t *testing.T) {
	handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "a", "drv": "postgres", "url": "host=127.0.0.1 dbname=sssghssssetdata user=tpt password=extreme sslmode=disable"})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "sssghssssetdata") {
		t.Error("excepted error contains [sssghssssetdata], but actual is", e)
	}
}

func TestDbHandlerAuthError(t *testing.T) {
	handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "a", "drv": "postgres", "url": "host=127.0.0.1 dbname=tpt_data user=tpt password=wwextreme sslmode=disable"})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "Password") {
		t.Error("excepted error contains [Password], but actual is", e)
	}
}

func dbTest(t *testing.T, cb func(backend *sql.DB)) {
	db, e := sql.Open(*db_drv, *db_url)
	if nil != e {
		t.Error(e)
		return
	}
	defer db.Close()

	_, e = db.Exec(`
DROP TABLE IF EXISTS tpt_test_for_handler;

CREATE TABLE IF NOT EXISTS tpt_test_for_handler (
  id                BIGSERIAL  PRIMARY KEY,
  priority          int DEFAULT 0,
  queue             varchar(200)
);`)

	if nil != e {
		t.Error(e)
		return
	}
	cb(db)
}

func TestDbHandlerScriptError(t *testing.T) {
	handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "insert aa"})
	if nil != e {
		t.Error(e)
		return
	}

	e = handler.Perform()
	if nil == e {
		t.Error("excepted error is not nil, but actual is nil")
		return
	}

	if !strings.Contains(e.Error(), "scanner_yyerror") {
		t.Error("excepted error contains [scanner_yyerror], but actual is", e)
	}
}

func assertCount(t *testing.T, db *sql.DB, sql string, excepted int64) {
	count := int64(-1)
	e := db.QueryRow(sql).Scan(&count)
	if nil != e {
		t.Error(e)
		return
	}

	if count != excepted {
		t.Error("excepted \"", sql, "\" is ", excepted, ", actual is ", count)
	}
}

func TestDbHandlerSimple(t *testing.T) {
	dbTest(t, func(db *sql.DB) {
		handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "insert into tpt_test_for_handler(priority, queue) values(12, 'sss')"})
		if nil != e {
			t.Error(e)
			return
		}

		e = handler.Perform()
		if nil != e {
			t.Error(e)
			return
		}

		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler WHERE priority = 12 and queue = 'sss'", 1)
		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler", 1)
	})
}

func TestDbHandlerMuti(t *testing.T) {
	dbTest(t, func(db *sql.DB) {
		handler, e := newDbHandler(map[string]interface{}{}, map[string]interface{}{"script": "insert into tpt_test_for_handler(priority, queue) values(12, 'sss'); insert into tpt_test_for_handler(priority, queue) values(112, 'aa')"})
		if nil != e {
			t.Error(e)
			return
		}

		e = handler.Perform()
		if nil != e {
			t.Error(e)
			return
		}

		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler WHERE priority = 12 and queue = 'sss'", 1)
		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler WHERE priority = 112 and queue = 'aa'", 1)
		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler", 2)
	})
}

func TestDbHandlerArguments(t *testing.T) {
	dbTest(t, func(db *sql.DB) {
		handler, e := newDbHandler(map[string]interface{}{},
			map[string]interface{}{"arguments": map[string]interface{}{"priority1": 23, "queue1": "q1", "priority2": 24, "queue2": "q2"},
				"script": `insert into tpt_test_for_handler(priority, queue) values({{.priority1}}, '{{.queue1}}'); 
			             insert into tpt_test_for_handler(priority, queue) values({{.priority2}}, '{{.queue2}}');`})
		if nil != e {
			t.Error(e)
			return
		}

		e = handler.Perform()
		if nil != e {
			t.Error(e)
			return
		}

		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler WHERE priority = 23 and queue = 'q1'", 1)
		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler WHERE priority = 24 and queue = 'q2'", 1)
		assertCount(t, db, "SELECT count(*) FROM tpt_test_for_handler", 2)
	})
}
