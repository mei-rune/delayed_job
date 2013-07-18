package delayed_job

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"flag"
	_ "github.com/lib/pq"
	"strconv"
	"time"
)

var (
	db_url = flag.String("data_db.url", "host=127.0.0.1 dbname=tpt_data user=tpt password=extreme sslmode=disable", "the db url")
	db_drv = flag.String("data_db.name", "postgres", "the db driver")

	is_test_for_lock = false
	test_ch_for_lock = make(chan int)
)

func IsNumericParams(drv string) bool {
	switch drv {
	case "postgres":
		return true
	default:
		return false
	}
}

// NullTime represents an time that may be null.
// NullTime implements the Scanner interface so
// it can be used as a scan destination, similar to NullTime.
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Int64 is not NULL
}

// Scan implements the Scanner interface.
func (n *NullTime) Scan(value interface{}) error {
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}

	n.Time, n.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

// A job object that is persisted to the database.
// Contains the work object as a YAML field.
type dbBackend struct {
	drv             string
	db              *sql.DB
	isNumericParams bool
}

func newBackend(drv, url string) (*dbBackend, error) {
	db, e := sql.Open(drv, url)
	if nil != e {
		return nil, e
	}
	return &dbBackend{drv: drv, db: db, isNumericParams: IsNumericParams(drv)}, nil
}

func (self *dbBackend) Close() {
	self.db.Close()
}

func (self *dbBackend) enqueue(priority int, queue string, run_at time.Time, args map[string]interface{}) error {
	job, e := newJob(self, priority, queue, run_at, args)
	if nil != e {
		return e
	}

	if *delay_jobs {
		return self.create(job)
	} else {
		return job.invokeJob()
	}
}

// When a worker is exiting, make sure we don't have any locked jobs.
func (self *dbBackend) clearLocks(worker_name string) error {
	var e error
	if self.isNumericParams {
		_, e = self.db.Exec("UPDATE tpt_delayed_jobs SET locked_by = NULL, locked_at = NULL WHERE locked_by = $1", worker_name)
	} else {
		_, e = self.db.Exec("UPDATE tpt_delayed_jobs SET locked_by = NULL, locked_at = NULL WHERE locked_by = ?", worker_name)
	}
	return e
}

func (self *dbBackend) reserve(w *worker) (*Job, error) {
	var buffer bytes.Buffer

	//buffer.WriteString("SELECT id, priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at FROM tpt_delayed_jobs")
	buffer.WriteString("SELECT id, priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at FROM tpt_delayed_jobs")
	if self.isNumericParams {
		buffer.WriteString(" WHERE (run_at <= $1 AND (locked_at IS NULL OR locked_at < $2) OR locked_by = $3) AND failed_at IS NULL")
	} else {
		buffer.WriteString(" WHERE (run_at <= ? AND (locked_at IS NULL OR locked_at < ?) OR locked_by = ?) AND failed_at IS NULL")
	}

	// scope to filter to the single next eligible job
	if -1 != w.min_priority {
		buffer.WriteString(" AND priority >= ")
		buffer.WriteString(strconv.FormatInt(int64(w.min_priority), 10))
	}

	if -1 != w.max_priority {
		buffer.WriteString(" AND priority <= ")
		buffer.WriteString(strconv.FormatInt(int64(w.max_priority), 10))
	}
	if nil != w.queues {
		switch len(w.queues) {
		case 0:
		case 1:
			buffer.WriteString(" AND queue = '")
			buffer.WriteString(w.queues[0])
			buffer.WriteString("'")
		default:
			buffer.WriteString(" AND queue in (")
			for i, s := range w.queues {
				if 0 != i {
					buffer.WriteString(", '")
				} else {
					buffer.WriteString("'")
				}

				buffer.WriteString(s)
				buffer.WriteString("'")
			}
			buffer.WriteString(")")
		}
	}
	buffer.WriteString(" ORDER BY priority ASC, run_at ASC")

	now := self.db_time_now()
	rows, e := self.db.Query(buffer.String(), now, now.Truncate(w.max_run_time), w.name)
	if nil != e {
		if sql.ErrNoRows == e {
			return nil, nil
		}
		return nil, e
	}
	defer rows.Close()

	for rows.Next() {
		job := &Job{}
		var queue sql.NullString
		var last_error sql.NullString
		var run_at NullTime
		var locked_at NullTime
		var failed_at NullTime
		var locked_by sql.NullString

		e = rows.Scan(
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
			return nil, e
		}

		if is_test_for_lock {
			test_ch_for_lock <- 1
			<-test_ch_for_lock
		}

		var c int64
		var result sql.Result
		if self.isNumericParams {
			result, e = self.db.Exec("UPDATE tpt_delayed_jobs SET locked_at = $1, locked_by = $2 WHERE id = $3 AND (locked_at IS NULL OR locked_at < $4 OR locked_by = $5) AND failed_at IS NULL", now, w.name, job.id, now.Truncate(w.max_run_time), w.name)
		} else {
			result, e = self.db.Exec("UPDATE tpt_delayed_jobs SET locked_at = ?, locked_by = ? WHERE id = ? AND (locked_at IS NULL OR locked_at < ? OR locked_by = ?) AND failed_at IS NULL", now, w.name, job.id, now.Truncate(w.max_run_time), w.name)
		}
		if nil != e {
			return nil, e
		}

		c, e = result.RowsAffected()
		if nil != e {
			return nil, e
		}

		if c > 0 {
			if queue.Valid {
				job.queue = queue.String
			}

			if last_error.Valid {
				job.last_error = last_error.String
			}

			if run_at.Valid {
				job.run_at = run_at.Time
			}

			if locked_at.Valid {
				job.locked_at = locked_at.Time
			}

			if failed_at.Valid {
				job.failed_at = failed_at.Time
			}

			if locked_by.Valid {
				job.locked_by = locked_by.String
			}

			job.backend = self
			return job, nil
		}
	}

	e = rows.Err()
	if nil != e {
		return nil, e
	}

	return nil, nil

	//     ready_scope.limit(worker.read_ahead).detect do |job|
	//     count = ready_scope.where(:id => job.id).update_all(:locked_at => now, :locked_by => worker.name)
	//     count == 1 && job.reload
	//   }

	// now = self.db_time_now

	// // Optimizations for faster lookups on some common databases
	// switch *drv  {
	// when "postgres":
	//   // Custom SQL required for PostgreSQL because postgres does not support UPDATE...LIMIT
	//   // This locks the single record 'FOR UPDATE' in the subquery (http://www.postgresql.org/docs/9.0/static/sql-select.html//SQL-FOR-UPDATE-SHARE)
	//   // Note: active_record would attempt to generate UPDATE...LIMIT like sql for postgres if we use a .limit() filter, but it would not use
	//   // 'FOR UPDATE' and we would have many locking conflicts
	//   subquery_sql      = ready_scope.limit(1).lock(true).select('id').to_sql
	//   reserved          = self.find_by_sql(["UPDATE tpt_delayed_jobs SET locked_at = ?, locked_by = ? WHERE id IN (select id from tpt_delayed_jobs " + buffer.+") RETURNING *", now, worker.name])
	//   reserved[0]
	// case "MySQL", "Mysql2":
	//   // This works on MySQL and possibly some other DBs that support UPDATE...LIMIT. It uses separate queries to lock and return the job
	//   count = ready_scope.limit(1).update_all(:locked_at => now, :locked_by => worker.name)
	//   return nil if count == 0
	//   self.where(:locked_at => now, :locked_by => worker.name, :failed_at => nil).first
	// case "MSSQL":
	//   // The MSSQL driver doesn't generate a limit clause when update_all is called directly
	//   subsubquery_sql = ready_scope.limit(1).to_sql
	//   // select("id") doesn't generate a subquery, so force a subquery
	//   subquery_sql = "SELECT id FROM (//{subsubquery_sql}) AS x"
	//   quoted_table_name = self.connection.quote_table_name(self.table_name)
	//   sql = ["UPDATE tpt_delayed_jobs SET locked_at = ?, locked_by = ? WHERE id IN (//{subquery_sql})", now, worker.name]
	//   count = self.connection.execute(sanitize_sql(sql))
	//   return nil if count == 0
	//   // MSSQL JDBC doesn't support OUTPUT INSERTED.* for returning a result set, so query locked row
	//   self.where(:locked_at => now, :locked_by => worker.name, :failed_at => nil).first
	// default:
	//   // This is our old fashion, tried and true, but slower lookup
	//   ready_scope.limit(worker.read_ahead).detect do |job|
	//     count = ready_scope.where(:id => job.id).update_all(:locked_at => now, :locked_by => worker.name)
	//     count == 1 && job.reload
	//   }
	// }
}

// Get the current time (GMT or local depending on DB)
// Note: This does not ping the DB to get the time, so all your clients
// must have syncronized clocks.
func (self *dbBackend) db_time_now() time.Time {
	return time.Now()
}

func (self *dbBackend) create(job *Job) error {
	var e error
	now := self.db_time_now()

	// var queue sql.NullString
	// if 0 == len(job.queue) {
	// 	queue.Valid = false
	// } else {
	// 	queue.Valid = true
	// 	queue.String = job.queue
	// }

	//1         2         3      4        5           NULL        6       NULL       NULL       NULL       7           8
	//priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at
	if self.isNumericParams {
		_, e = self.db.Exec("INSERT INTO tpt_delayed_jobs(priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NULL, $6, NULL, NULL, NULL, $7, $8)",
			job.priority, job.attempts, job.queue, job.handler, job.handler_id, now, now, now)
	} else {
		_, e = self.db.Exec("INSERT INTO tpt_delayed_jobs(priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, NULL, NULL, NULL, ?, ?)",
			job.priority, job.attempts, job.queue, job.handler, job.handler_id, now, now, now)
	}
	if nil != e {
		return e
	}

	return nil
}

func (self *dbBackend) update(id int64, attributes map[string]interface{}) error {
	var buffer bytes.Buffer
	params := make([]interface{}, 0, len(attributes))

	buffer.WriteString("UPDATE tpt_delayed_jobs SET ")

	for k, v := range attributes {
		if 0 != len(params) {
			buffer.WriteString(", ")
		}
		buffer.WriteString(k)

		if nil == v {
			buffer.WriteString(" = NULL")
		} else {
			if self.isNumericParams {
				buffer.WriteString(" = $")
				buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
			} else {
				buffer.WriteString(" = ?")
			}

			params = append(params, v)
		}
	}

	if 0 != len(params) {
		buffer.WriteString(", ")
	}
	if self.isNumericParams {
		buffer.WriteString("updated_at = $")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
	} else {
		buffer.WriteString("updated_at = ?")
	}
	params = append(params, self.db_time_now())

	if self.isNumericParams {
		buffer.WriteString(" WHERE id = $")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
	} else {
		buffer.WriteString(" WHERE id = ?")
	}
	params = append(params, id)

	//fmt.Println(buffer.String())
	//fmt.Println(params)
	_, e := self.db.Exec(buffer.String(), params...)
	if nil != e && sql.ErrNoRows != e {
		return e
	}
	return nil
}

func (self *dbBackend) destroy(id int64) error {
	var e error
	if self.isNumericParams {
		_, e = self.db.Exec("DELETE FROM tpt_delayed_jobs WHERE id = $1", id)
	} else {
		_, e = self.db.Exec("DELETE FROM tpt_delayed_jobs WHERE id = ?", id)
	}

	if nil != e && sql.ErrNoRows != e {
		return e
	}
	return nil
}
