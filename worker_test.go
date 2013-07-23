package delayed_job

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func workTest(t *testing.T, cb func(w *worker, backend *dbBackend)) {
	w, e := newWorker(map[string]interface{}{})
	if nil != e {
		t.Error(e)
		return
	}
	defer w.Close()

	_, e = w.backend.db.Exec(`
DROP TABLE IF EXISTS tpt_delayed_jobs;

CREATE TABLE IF NOT EXISTS tpt_delayed_jobs (
  id                BIGSERIAL  PRIMARY KEY,
  priority          int DEFAULT 0,
  attempts          int DEFAULT 0,
  queue             varchar(200),
  handler           text  NOT NULL,
  handler_id        varchar(200)  NOT NULL,
  last_error        varchar(2000),
  run_at            timestamp with time zone,
  locked_at         timestamp with time zone,
  failed_at         timestamp with time zone,
  locked_by         varchar(200),
  created_at        timestamp with time zone  NOT NULL,
  updated_at        timestamp with time zone NOT NULL,

  CONSTRAINT tpt_delayed_jobs_unique_handler_id UNIQUE (handler_id)
);`)
	if nil != e {
		t.Error(e)
		return
	}

	w.start()

	cb(w, w.backend)
}

func TestWorker(t *testing.T) {
	workTest(t, func(w *worker, backend *dbBackend) {
		time.Sleep(1 * time.Second)

		_, _, e := w.work_off(10)
		if nil != e {
			t.Error(e)
		}
	})
}

func TestRunJob(t *testing.T) {
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		time.Sleep(1 * time.Second)

		select {
		case <-test_chan:
			return
		default:
			t.Error("not recv")
		}
	})
}

func TestRunError(t *testing.T) {
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a"})
		if nil != e {
			t.Error(e)
			return
		}

		time.Sleep(1 * time.Second)
		select {
		case <-test_chan:
		case <-time.After(1 * time.Second):
			t.Error("not recv")
		}

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM tpt_delayed_jobs")

		var attempts int64
		var run_at NullTime
		var locked_at NullTime
		var locked_by sql.NullString
		var handler sql.NullString
		var last_error sql.NullString

		e = row.Scan(&attempts, &run_at, &locked_at, &locked_by, &handler, &last_error)
		if nil != e {
			t.Error(e)
			return
		}

		if !run_at.Valid {
			t.Error("excepted run_at is valid, actual is invalid")
		}
		if locked_at.Valid {
			t.Error("excepted locked_at is invalid, actual is valid - ", locked_at.Time)
		}
		if locked_by.Valid {
			t.Error("excepted locked_by is invalid, actual is valid - ", locked_by.String)
		}

		if !handler.Valid {
			t.Error("excepted handler is not empty, actual is invalid")
		}

		if !last_error.Valid {
			t.Error("excepted last_error is not empty, actual is invalid")
		}

		if 1 != attempts {
			t.Error("excepted attempts is '1', and actual is ", attempts)
		}

		if (time.Now().Unix() + 5 - run_at.Time.Unix()) < 1 {
			t.Error("excepted run_at is ", run_at.Time, ", actual is", time.Now())
		}

		if !strings.Contains(handler.String, "\"type\": \"test\"") {
			t.Error("excepted handler contains '\"type\": \"test\"', actual is ", handler.String)
		}

		if "throw a" != last_error.String {
			t.Error("excepted run_at is 'throw a', actual is", last_error.String)
		}

	})
}

func TestRunFailed(t *testing.T) {
	*default_destroy_failed_jobs = false
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a", "max_attempts": "1"})
		if nil != e {
			t.Error(e)
			return
		}

		time.Sleep(1 * time.Second)

		select {
		case <-test_chan:
		default:
			t.Error("not recv")
		}

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM tpt_delayed_jobs")

		var attempts int64
		var run_at NullTime
		var locked_at NullTime
		var locked_by sql.NullString
		var handler sql.NullString
		var last_error sql.NullString

		e = row.Scan(&attempts, &run_at, &locked_at, &locked_by, &handler, &last_error)
		if nil != e {
			t.Error(e)
			return
		}

		if !run_at.Valid {
			t.Error("excepted run_at is valid, actual is invalid")
		}
		// if locked_at.Valid {
		// 	t.Error("excepted locked_at is invalid, actual is valid - ", locked_at.Time)
		// }
		// if locked_by.Valid {
		// 	t.Error("excepted locked_by is invalid, actual is valid - ", locked_by.String)
		// }

		if !handler.Valid {
			t.Error("excepted handler is not empty, actual is invalid")
		}

		if !last_error.Valid {
			t.Error("excepted last_error is not empty, actual is invalid")
		}

		// if 1 != attempts {
		// 	t.Error("excepted attempts is '1', and actual is ", attempts)
		// }

		if (time.Now().Unix() + 5 - run_at.Time.Unix()) < 1 {
			t.Error("excepted run_at is ", run_at.Time, ", actual is", time.Now())
		}

		if !strings.Contains(handler.String, "\"type\": \"test\"") {
			t.Error("excepted handler contains '\"type\": \"test\"', actual is ", handler.String)
		}

		if "throw a" != last_error.String {
			t.Error("excepted run_at is 'throw a', actual is", last_error.String)
		}

	})
}

func TestRunFailedAndDestoryIt(t *testing.T) {
	*default_destroy_failed_jobs = true
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a", "max_attempts": "1"})
		if nil != e {
			t.Error(e)
			return
		}

		time.Sleep(1 * time.Second)

		select {
		case <-test_chan:
		default:
			t.Error("not recv")
		}

		var count int64
		e = backend.db.QueryRow("SELECT count(*) FROM tpt_delayed_jobs").Scan(&count)
		if nil != e {
			t.Error(e)
			return
		}

		if 0 != count {
			t.Error("excepted jobs is empty, actual is", count)
		}

	})
}
