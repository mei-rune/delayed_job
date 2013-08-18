package delayed_job

import (
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"
)

func workTest(t *testing.T, cb func(w *worker, backend *dbBackend)) {
	*default_sleep_delay = 1 * time.Second
	WorkTest(t, func(w *TestWorker) {
		w.start()
		defer w.Close()

		cb(w.worker, w.backend)
	})
}

func TestWork(t *testing.T) {
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

		select {
		case <-test_chan:
			return
		case <-time.After(2 * time.Second):
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

		select {
		case <-test_chan:
		case <-time.After(2 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(5000 * time.Millisecond)

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM " + *table_name)

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

		//if !strings.Contains(*db_drv, "mysql") {
		now := backend.db_time_now()
		if math.Abs(float64(now.Unix()+5-run_at.Time.Unix())) < 1 {
			t.Error("excepted run_at is ", run_at.Time, ", actual is", now)
		}
		//}
		if !strings.Contains(handler.String, "\"type\": \"test\"") {
			t.Error("excepted handler contains '\"type\": \"test\"', actual is ", handler.String)
		}

		if "throw a" != last_error.String {
			t.Error("excepted run_at is 'throw a', actual is", last_error.String)
		}

	})
}

func TestRunFailedAndNotDestoryIt2(t *testing.T) {
	*default_destroy_failed_jobs = false
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a", "max_attempts": "1"})
		if nil != e {
			t.Error(e)
			return
		}

		select {
		case <-test_chan:
		case <-time.After(4 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(1500 * time.Millisecond)

		rows, e := backend.db.Query("SELECT last_error FROM " + *table_name)
		if nil != e {
			t.Error(e)
			return
		}

		for rows.Next() {
			var last_error sql.NullString

			e = rows.Scan(&last_error)
			if nil != e {
				t.Error(e)
				return
			}

			if !last_error.Valid {
				t.Error("excepted last_error is not empty, actual is invalid")
			}

			if last_error.Valid && "throw a" != last_error.String {
				t.Error("excepted run_at is 'throw a', actual is", last_error.String)
			}
		}
	})
}

func TestRunFailedAndNotDestoryIt(t *testing.T) {
	*default_destroy_failed_jobs = false
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a", "max_attempts": "1"})
		if nil != e {
			t.Error(e)
			return
		}

		select {
		case <-test_chan:
		case <-time.After(4 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(1500 * time.Millisecond)

		rows, e := backend.db.Query("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM " + *table_name)
		if nil != e {
			t.Error(e)
			return
		}

		for rows.Next() {
			var attempts int64
			var run_at NullTime
			var locked_at NullTime
			var locked_by sql.NullString
			var handler sql.NullString
			var last_error sql.NullString

			e = rows.Scan(&attempts, &run_at, &locked_at, &locked_by, &handler, &last_error)
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

			//if !strings.Contains(*db_drv, "mysql") {
			now := backend.db_time_now()
			if interval := now.Sub(run_at.Time); interval < 1*time.Second {
				t.Error("excepted run_at is ", now, ", actual is", run_at.Time, "interval is ", interval)
			}
			//}

			if !strings.Contains(handler.String, "\"type\": \"test\"") {
				t.Error("excepted handler contains '\"type\": \"test\"', actual is ", handler.String)
			}

			if last_error.Valid && "throw a" != last_error.String {
				t.Error("excepted run_at is 'throw a', actual is", last_error.String)
			}
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

		select {
		case <-test_chan:
		case <-time.After(2 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(1500 * time.Millisecond)

		var count int64
		e = backend.db.QueryRow("SELECT count(*) FROM " + *table_name).Scan(&count)
		if nil != e {
			t.Error(e)
			return
		}

		if 0 != count {
			t.Error("excepted jobs is empty, actual is", count)
		}

	})
}
