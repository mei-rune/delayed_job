package delayed_job

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPush(t *testing.T) {
	backendTest(t, func(backend *dbBackend) {

		srv := httptest.NewServer(&webFront{nil, backend})
		defer srv.Close()

		var buffer bytes.Buffer

		e := json.NewEncoder(&buffer).Encode(map[string]interface{}{
			"priority": 1,
			"queue":    "aa",
			"run_at":   time.Time{},
			"handler":  map[string]interface{}{"type": "test"}})
		if nil != e {
			t.Error(e)
			return
		}

		resp, e := http.Post(srv.URL+"/push", "application/json", &buffer)
		if nil != e {
			t.Error(e)
			return
		}

		if resp.StatusCode != http.StatusOK {
			bs, e := ioutil.ReadAll(resp.Body)
			if nil != e {
				t.Errorf("%v: error", resp.StatusCode)
			} else {
				t.Errorf("%v: %v", resp.StatusCode, string(bs))
			}
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
		var handler NullString

		e = row.Scan(
			&job.id,
			&job.priority,
			&job.attempts,
			&queue,
			&handler,
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

		if handler.Valid {
			job.handler = handler.String
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

		if last_error.Valid {
			t.Error("excepted last_error is invalid, actual is ", last_error.String)
		}

		if locked_at.Valid && !locked_at.Time.IsZero() {
			t.Error("excepted locked_at is invalid actual is ", locked_at.Time)
		}

		if failed_at.Valid && !failed_at.Time.IsZero() {
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
