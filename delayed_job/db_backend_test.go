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
	backend, e := newBackend(*db_drv, *db_url)
	if nil != e {
		t.Error(e)
		return
	}
	defer backend.Close()

	_, e = backend.db.Exec(`
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
	cb(backend)
}

func TestEnqueue(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 12, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		row := backend.db.QueryRow("SELECT id, priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at FROM tpt_delayed_jobs")

		job := &Job{}
		var queue sql.NullString
		var last_error sql.NullString
		var run_at NullTime
		var locked_at NullTime
		var failed_at NullTime
		var locked_by sql.NullString

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
			&job.created_at,
			&job.updated_at)
		if nil != e {
			t.Error(e)
			return
		}

		if job.priority != 1 {
			t.Error("excepted priority is 1, actual is ", job.priority)
		}
		if job.attempts != 12 {
			t.Error("excepted attempts is 12, actual is ", job.attempts)
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

		if last_error.Valid {
			t.Error("excepted last_error is invalid, actual is ", last_error.String)
		}

		if locked_at.Valid {
			t.Error("excepted locked_at is invalid actual is ", locked_at.Time)
		}

		if failed_at.Valid {
			t.Error("excepted failed_at is invalid, actual is ", failed_at.Time)
		}

		if locked_by.Valid {
			t.Error("excepted locked_by is invalid, actual is ", locked_by.String)
		}

		select {
		case <-test_chan:
			t.Error("unexcepted recv")
		default:
		}
	})
}

func TestGet(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 12, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}
		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123"}
		job, e := backend.reserve(w)
		if nil != e {
			t.Error(e)
			return
		}

		if nil == job {
			t.Error("excepted job is not nil, actual is nil")
			return
		}

		row := backend.db.QueryRow("SELECT locked_at, locked_by FROM tpt_delayed_jobs where id = " + strconv.FormatInt(job.id, 10))

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

		if math.Abs(float64(locked_at.Time.Unix()-time.Now().Unix())) > 10 {
			t.Error("excepted locked_at is now, actual is", locked_at.Time)
		}
		if w.name != locked_by.String {
			t.Error("excepted locked_at is now, actual is", locked_at.Time)
		}

		if job.priority != 1 {
			t.Error("excepted priority is 1, actual is ", job.priority)
		}
		if job.attempts != 12 {
			t.Error("excepted attempts is 12, actual is ", job.attempts)
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

		if !job.locked_at.IsZero() {
			t.Error("excepted locked_at is invalid actual is ", job.locked_at)
		}

		if !job.failed_at.IsZero() {
			t.Error("excepted failed_at is invalid, actual is ", job.failed_at)
		}

		if "aa_pid:123" != job.locked_by {
			t.Error("excepted locked_by is invalid, actual is ", job.locked_by)
		}

		select {
		case <-test_chan:
			t.Error("unexcepted recv")
		default:
		}
	})
}

func TestGetWithLocked(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 12, "aa", time.Time{}, map[string]interface{}{"type": "test"})
		if nil != e {
			t.Error(e)
			return
		}

		_, e = backend.db.Exec("UPDATE tpt_delayed_jobs SET locked_at = now(), locked_by = 'aa'")
		if nil != e {
			t.Error(e)
			return
		}

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123"}
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

func TestLockedInGet(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {
		e := backend.enqueue(1, 12, "aa", time.Time{}, map[string]interface{}{"type": "test"})
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

			_, e := backend.db.Exec("UPDATE tpt_delayed_jobs SET locked_at = now(), locked_by = 'aa'")
			if nil != e {
				t.Error(e)
			}
			test_ch_for_lock <- 1
		}()

		w := &worker{min_priority: -1, max_priority: -1, name: "aa_pid:123"}
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
