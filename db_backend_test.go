package delayed_job

import (
	"database/sql"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func backendTest(t *testing.T, cb func(backend *dbBackend)) {
	// old_mode := *run_mode
	// *run_mode = "init_db"
	// defer func() {
	// 	*run_mode = old_mode
	// }()
	e := Main(":0", "init_db")
	if nil != e {
		t.Error(e)
		return
	}

	backend, e := newBackend(*db_drv, *db_url, nil)
	if nil != e {
		t.Error(e)
		return
	}
	defer backend.Close()

	// 	_, e = backend.db.Exec(`
	// DROP TABLE IF EXISTS ` + *table_name + `;

	// CREATE TABLE IF NOT EXISTS ` + *table_name + ` (
	//   id                SERIAL  PRIMARY KEY,
	//   priority          int DEFAULT 0,
	//   attempts          int DEFAULT 0,
	//   queue             varchar(200),
	//   handler           text  NOT NULL,
	//   handler_id        varchar(200),
	//   last_error        varchar(2000),
	//   run_at            timestamp,
	//   locked_at         timestamp,
	//   failed_at         timestamp,
	//   locked_by         varchar(200),
	//   created_at        timestamp NOT NULL,
	//   updated_at        timestamp NOT NULL
	// );`)
	// 	if nil != e {
	// 		t.Error(e)
	// 		return
	// 	}
	cb(backend)
}

func TestEnqueue(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		row := backend.db.QueryRow("SELECT id, priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at FROM " + *table_name)

		job := &Job{}
		var queue sql.NullString
		var last_error sql.NullString
		var run_at NullTime
		var locked_at NullTime
		var failed_at NullTime
		var locked_by sql.NullString

		var created_at NullTime
		var updated_at NullTime

		e = row.Scan(
			&job.id,
			&job.priority,
			&job.attempts,
			&queue,
			&job.handler,
			&job.handler_id,
			&last_error,
			&run_at,
			&locked_at,
			&failed_at,
			&locked_by,
			&created_at,
			&updated_at)
		if nil != e {
			t.Error(e)
			return
		}

		if job.priority != 1 {
			t.Error("excepted priority is 1, actual is ", job.priority)
		}
		if job.attempts != 0 {
			t.Error("excepted attempts is 0, actual is ", job.attempts)
		}
		if queue.Valid && queue.String != "aa" {
			t.Error("excepted queue is 'aa', actual is ", queue.String)
		}
		if !strings.Contains(job.handler, "\"type\": \"test\"") {
			t.Error("excepted handler is 'aa', actual is ", job.handler)
		}
		if 0 == len(job.handler_id) {
			t.Error("excepted handler_id is not empty, actual is ", job.handler_id)
		}

		if last_error.Valid && 0 != len(last_error.String) {
			t.Error("excepted last_error is valid, actual is ", last_error.String)
		}

		if locked_at.Valid && !locked_at.Time.IsZero() && locked_at.Time.Format("2006-01-02") != "0001-01-01" {
			t.Error("excepted locked_at is invalid actual is ", locked_at.Time)
		}

		if failed_at.Valid && !failed_at.Time.IsZero() && failed_at.Time.Format("2006-01-02") != "0001-01-01" {
			t.Error("excepted failed_at is invalid, actual is ", failed_at.Time)
		}

		if locked_by.Valid && 0 != len(locked_by.String) {
			t.Error("excepted locked_by is invalid, actual is ", locked_by.String)
		}

		if !created_at.Valid || created_at.Time.IsZero() || created_at.Time.Format("2006-01-02") == "0001-01-01" {
			t.Error("excepted created_at is invalid actual is ", created_at.Time)
		}

		if !updated_at.Valid || updated_at.Time.IsZero() || updated_at.Time.Format("2006-01-02") == "0001-01-01" {
			t.Error("excepted updated_at is valid, actual is ", updated_at.Time)
		}

		select {
		case <-test_chan:
			t.Error("unexcepted recv")
		default:
		}
	})
}

func TestGetSimple(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		for i := 0; i < 10; i++ {
			e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
			if nil != e {
				t.Error(e)
				return
			}
		}

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil == job {
			t.Error("excepted job is not nil, actual is nil")
			return
		}

		row := backend.db.QueryRow("SELECT locked_at, locked_by FROM " + *table_name + " where id = " + strconv.FormatInt(job.id, 10))

		var locked_at NullTime
		var locked_by sql.NullString

		e = row.Scan(&locked_at, &locked_by)
		if nil != e {
			t.Error(e)
			return
		}

		if !locked_at.Valid {
			t.Error("excepted locked_at is not empty, actual is invalid")
		}

		if !locked_by.Valid {
			t.Error("excepted locked_by is not empty, actual is invalid")
		}

		if math.Abs(float64(locked_at.Time.Unix()-backend.db_time_now().Unix())) > 10 {
			t.Log(locked_at.Time, backend.db_time_now())
			t.Error("excepted locked_at is now, actual is", locked_at.Time)
		}
		if w.name != locked_by.String {
			t.Error("excepted locked_at is now, actual is", locked_at.Time)
		}

		if job.priority != 1 {
			t.Error("excepted priority is 1, actual is ", job.priority)
		}
		if job.attempts != 0 {
			t.Error("excepted attempts is 0, actual is ", job.attempts)
		}
		if job.queue != "aa" {
			t.Error("excepted queue is 'aa', actual is ", job.queue)
		}
		if !strings.Contains(job.handler, "\"type\": \"test\"") {
			t.Error("excepted handler is 'aa', actual is ", job.handler)
		}
		if 0 == len(job.handler_id) {
			t.Error("excepted handler_id is not empty, actual is ", job.handler_id)
		}

		if job.last_error != "" {
			t.Error("excepted last_error is invalid, actual is ", job.last_error)
		}

		if !job.failed_at.IsZero() {
			t.Error("excepted failed_at is invalid, actual is ", job.failed_at)
		}

		if "postgres" != backend.drv {
			if !job.locked_at.IsZero() {
				t.Error("excepted locked_at is invalid actual is ", job.locked_at)
			}

			if "" != job.locked_by {
				t.Error("excepted locked_by is invalid, actual is ", job.locked_by)
			}
		}

		select {
		case <-test_chan:
			t.Error("unexcepted recv")
		default:
		}

		row = backend.db.QueryRow("SELECT count(*) FROM " + *table_name + " where  locked_by is NULL AND locked_at is NULL")
		var count int = 0
		e = row.Scan(&count)
		if nil != e {
			t.Error(e)
			return
		}

		if 9 != count {
			t.Error("excepted read 1, actual is ", 10-count)
		}
	})
}

func TestGetWithLocked(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		if strings.Contains(*db_drv, "odbc_with_mssql") {
			_, e = backend.db.Exec("UPDATE " + *table_name + " SET locked_at = SYSUTCDATETIME(), locked_by = 'aa'")
		} else {
			_, e = backend.db.Exec("UPDATE " + *table_name + " SET locked_at = now(), locked_by = 'aa'")
		}
		if nil != e {
			t.Error(e)
			return
		}

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil != job {
			t.Error("excepted job is nil, actual is not nil")
			return
		}
	})
}

func TestGetWithFailed(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		if strings.Contains(*db_drv, "odbc_with_mssql") {
			_, e = backend.db.Exec("UPDATE " + *table_name + " SET failed_at = SYSUTCDATETIME(), last_error = 'aa'")
		} else {
			_, e = backend.db.Exec("UPDATE " + *table_name + " SET failed_at = now(), last_error = 'aa'")
		}
		if nil != e {
			t.Error(e)
			return
		}

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil != job {
			t.Error("excepted job is nil, actual is not nil")
			return
		}
	})
}

func TestLockedJobInGet(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		if "postgres" == backend.drv {
			t.Skip("postgres is skipped.")
		}

		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		is_test_for_lock = true
		defer func() {
			is_test_for_lock = false
		}()

		go func() {
			<-test_ch_for_lock

			var e error
			if strings.Contains(*db_drv, "odbc_with_mssql") {
				_, e = backend.db.Exec("UPDATE " + *table_name + " SET locked_at = SYSUTCDATETIME(), locked_by = 'aa'")
			} else {
				_, e = backend.db.Exec("UPDATE " + *table_name + " SET locked_at = now(), locked_by = 'aa'")
			}
			if nil != e {
				t.Error(e)
			}
			test_ch_for_lock <- 1
		}()

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil != job {
			t.Error("excepted job is nil, actual is not nil")
			return
		}
	})
}

func TestFailedJobInGet(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		if "postgres" == backend.drv {
			t.Skip("postgres is skipped.")
		}

		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		is_test_for_lock = true
		defer func() {
			is_test_for_lock = false
		}()

		go func() {
			<-test_ch_for_lock

			var e error
			if strings.Contains(*db_drv, "odbc_with_mssql") {
				_, e = backend.db.Exec("UPDATE " + *table_name + " SET failed_at = SYSUTCDATETIME(), last_error = 'aa'")
			} else {
				_, e = backend.db.Exec("UPDATE " + *table_name + " SET failed_at = now(), last_error = 'aa'")
			}

			if nil != e {
				t.Error(e)
				return
			}
			test_ch_for_lock <- 1
		}()

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil != job {
			t.Error("excepted job is nil, actual is not nil")
			return
		}
	})
}

func TestDestory(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil == job {
			t.Error("excepted job is not nil, actual is nil")
			return
		}

		job.destroyIt()

		count := int64(-1)
		e = backend.db.QueryRow("SELECT count(*) FROM " + *table_name + "").Scan(&count)
		if nil != e {
			t.Error(e)
			return
		}

		if count != 0 {
			t.Error("excepted job is empty after destory it, actual is ", count)
			return
		}

	})
}

func TestFailIt(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil == job {
			t.Error("excepted job is not nil, actual is nil")
			return
		}

		e = job.failIt("1234")
		if nil != e {
			t.Error(e)
			return
		}

		row := backend.db.QueryRow("SELECT failed_at, last_error FROM " + *table_name + " where id = " + strconv.FormatInt(job.id, 10))

		var failed_at NullTime
		var last_error sql.NullString

		e = row.Scan(&failed_at, &last_error)
		if nil != e {
			t.Error(e)
			return
		}

		if !failed_at.Valid {
			t.Error("excepted failed_at is not empty, actual is invalid")
		} else if math.Abs(float64(failed_at.Time.Unix()-backend.db_time_now().Unix())) > 10 {
			t.Error("excepted failed_at is now, actual is", failed_at.Time)
		}

		if !last_error.Valid {
			t.Error("excepted last_error is not empty, actual is invalid")
		} else if "1234" != last_error.String {
			t.Error("excepted last_error is '1234', and actual is ", last_error.String)
		}

	})
}

func TestRescheduleIt(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123", max_run_time: 1 * time.Minute}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil == job {
			t.Error("excepted job is not nil, actual is nil")
			return
		}
		now := backend.db_time_now()
		job.will_update_attributes()["@handler"] = map[string]interface{}{"type": "test", "aa": "testsss"}
		e = job.rescheduleIt(now, "throw s")
		if nil != e {
			t.Error(e)
			return
		}

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM " + *table_name + " where id = " + strconv.FormatInt(job.id, 10))

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
		if locked_at.Valid && !locked_at.Time.IsZero() {
			t.Error("excepted locked_at is invalid, actual is valid")
		}
		if locked_by.Valid {
			t.Error("excepted locked_by is invalid, actual is valid")
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

		if math.Abs(float64(run_at.Time.Unix()-now.Unix())) > 2 {
			t.Error("excepted run_at is ", run_at.Time, ", actual is", now)
		}

		if !strings.Contains(handler.String, "\"type\": \"test\"") {
			t.Error("excepted handler contains '\"type\": \"test\"', actual is ", handler.String)
		}

		if !strings.Contains(handler.String, "\"aa\": \"testsss\"") {
			t.Error("excepted handler is '\"aa\": \"testsss\"', actual is ", handler.String)
		}

		if last_error.String != "throw s" {
			t.Error("excepted last_error is 'throw s', actual is ", last_error.String)
		}

	})
}
