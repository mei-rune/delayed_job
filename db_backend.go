package delayed_job

import (
	"bytes"
	"database/sql"
	"database/sql/driver"

	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitee.com/chunanyong/dm" // 达梦
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/ziutek/mymysql/godrv"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	AUTO       = 0
	POSTGRESQL = 1
	MYSQL      = 2
	MSSQL      = 3
	ORACLE     = 4
	DB2        = 5
	SYBASE     = 6
	DM         = 7
	KINGBASE   = 8
)

var (
	PreprocessArgs func(args interface{}) interface{}

	table_name = flag.String("db_table", "delayed_jobs", "the table name for jobs")

	is_test_for_lock = false
	test_ch_for_lock = make(chan int)

	select_sql_string = ""
	fields_sql_string = " id, priority, repeat_count, repeat_interval, attempts, max_attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at "
)

func preprocessArgs(args interface{}) interface{} {
	if nil != PreprocessArgs {
		return PreprocessArgs(args)
	}
	return args
}

func ToDbType(drv string) int {
	switch drv {
	case "kingbase":
		return KINGBASE
	case "dm":
		return DM
	case "postgres":
		return POSTGRESQL
	case "mysql", "mymysql":
		return MYSQL
	case "odbc_with_mssql", "mssql", "sqlserver":
		return MSSQL
	case "oci8", "odbc_with_oracle", "oracle", "ora":
		return ORACLE
	default:
		if strings.Contains(drv, "oracle") {
			return ORACLE
		}
		if strings.Contains(drv, "sqlserver") || strings.Contains(drv, "mssql") {
			return MSSQL
		}
		if strings.Contains(drv, "db2") {
			return DB2
		}
		if strings.Contains(drv, "sybase") {
			return SYBASE
		}
		return AUTO
	}
}
func initDB() {
	//flag.Set("db_type", fmt.Sprint(DbType(*db_drv)))
	createSQL()
}

func createSQL() {
	select_sql_string = "SELECT " + fields_sql_string + " FROM " + *table_name + " "
}

func SetTable(table_name string) {
	flag.Set("db_table", table_name)
	createSQL()
}

// func SetDbUrl(drv, url string) {
// 	flag.Set("db_url", url)
// 	flag.Set("db_drv", drv)
// 	initDB()
// }

func i18n(dbType int, drv string, e error) error {
	return I18n(dbType, drv, e)
}

func I18n(dbType int, drv string, e error) error {
	if ORACLE == dbType && "oci8" == drv {
		decoder := simplifiedchinese.GB18030.NewDecoder()
		msg, _, err := transform.String(decoder, e.Error())
		if nil == err {
			return errors.New(msg)
		}
	}
	return e
	// if ORACLE == dbType && "oci8" == drv {
	// 	return errors.New(decoder.ConvertString(e.Error()))
	// }
	// return e
}

func i18nString(dbType int, drv string, e error) string {
	if ORACLE == dbType && "oci8" == drv {
		decoder := simplifiedchinese.GB18030.NewDecoder()
		msg, _, err := transform.String(decoder, e.Error())
		if nil == err {
			return msg
		}
	}
	return e.Error()
	// if ORACLE == dbType && "oci8" == drv {
	// 	return decoder.ConvertString(e.Error())
	// }
	// return e.Error()
}

func IsNumericParams(dbType int) bool {
	switch dbType {
	case ORACLE, DM, POSTGRESQL, KINGBASE:
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
	// fmt.Println("wwwwwwwwwwwww", value)
	n.Time, n.Valid = value.(time.Time)
	if !n.Valid {
		if s, ok := value.(string); ok {
			var e error
			for _, layout := range []string{"2006-01-02 15:04:05.000000000", "2006-01-02 15:04:05.000000", "2006-01-02 15:04:05.000", "2006-01-02 15:04:05", "2006-01-02"} {
				if n.Time, e = time.ParseInLocation(layout, s, time.UTC); nil == e {
					n.Valid = true
					break
				}
			}
		}
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

type NullString struct {
	String string
	Valid  bool // Valid is true if Int64 is not NULL
}

// Scan implements the Scanner interface.
func (n *NullString) Scan(value interface{}) error {
	if value == nil {
		n.String, n.Valid = "", false
		return nil
	}
	switch s := value.(type) {
	case []byte:
		if s != nil {
			n.Valid = true
			n.String = string(s)
		} else {
			n.String, n.Valid = "", false
		}
		return nil
	case string:
		n.Valid = true
		n.String = s
		return nil
	case *[]byte:
		if s != nil && *s != nil {
			n.Valid = true
			n.String = string(*s)
		} else {
			n.String, n.Valid = "", false
		}
		return nil
	case *string:
		if s == nil {
			n.String, n.Valid = "", false
		} else {
			n.Valid = true
			n.String = *s
		}
		return nil
	case *dm.DmClob:
		l, err := s.GetLength()
		if err != nil {
			return err
		}
		if l == 0 {
			n.Valid = true
			return nil
		}
		n.String, err = s.ReadString(1, int(l))
		if err != nil {
			return err
		}
		n.Valid = true
		return nil
	}
	return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type NullString", value)
}

// Value implements the driver Valuer interface.
func (n NullString) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.String, nil
}

// A job object that is persisted to the database.
// Contains the work object as a YAML field.
type dbBackend struct {
	ctx             map[string]interface{}
	drv             string
	dbType          int
	db              *sql.DB
	isNumericParams bool
}

func newBackend(drvName, dbURL string, ctx map[string]interface{}) (*dbBackend, error) {
	if drvName == "" {
		return nil, errors.New("db_drv is missing")
	}
	if dbURL == "" {
		return nil, errors.New("db_url is missing")
	}

	drv := drvName
	if strings.HasPrefix(drvName, "odbc_with_") {
		drv = "odbc"
	}

	db, e := sql.Open(drv, dbURL)
	if nil != e {
		return nil, e
	}
	dbType := ToDbType(drv)
	return &dbBackend{ctx: ctx, drv: drv, db: db, dbType: dbType, isNumericParams: IsNumericParams(dbType)}, nil
}

func (self *dbBackend) Close() error {
	self.db.Close()
	return nil
}

func (self *dbBackend) enqueue(priority, repeat_count int, repeat_interval string, max_attempts int, queue string, run_at time.Time, args map[string]interface{}) error {
	job, e := newJob(self, priority, repeat_count, repeat_interval, max_attempts, queue, run_at, args, true)
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
		_, e = self.db.Exec("UPDATE "+*table_name+" SET locked_by = NULL, locked_at = NULL WHERE locked_by = $1", worker_name)
	} else {
		_, e = self.db.Exec("UPDATE "+*table_name+" SET locked_by = NULL, locked_at = NULL WHERE locked_by = ?", worker_name)
	}
	return i18n(self.dbType, self.drv, e)
}

func (self *dbBackend) readJobFromRow(row interface {
	Scan(dest ...interface{}) error
}) (*Job, error) {
	job := &Job{}
	var queue sql.NullString
	var handler_id sql.NullString
	var repeat_interval sql.NullString
	var last_error NullString
	var attempts sql.NullInt64
	var run_at NullTime
	var locked_at NullTime
	var failed_at NullTime
	var locked_by sql.NullString
	var created_at NullTime
	var updated_at NullTime
	var handler NullString

	e := row.Scan(
		&job.id,
		&job.priority,
		&job.repeat_count,
		&repeat_interval,
		&attempts,
		&job.max_attempts,
		&queue,
		&handler,
		&handler_id,
		&last_error,
		&run_at,
		&locked_at,
		&failed_at,
		&locked_by,
		&created_at,
		&updated_at)
	if nil != e {
		return nil, errors.New("scan job failed from the database, " + i18nString(self.dbType, self.drv, e))
	}

	if queue.Valid {
		job.queue = queue.String
	}

	if handler.Valid {
		job.handler = handler.String
	}

	if attempts.Valid {
		job.attempts = int(attempts.Int64)
	}

	if handler_id.Valid {
		job.handler_id = handler_id.String
	}

	if repeat_interval.Valid {
		job.repeat_interval = repeat_interval.String
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

	if created_at.Valid {
		job.created_at = created_at.Time
	}

	if updated_at.Valid {
		job.updated_at = updated_at.Time
	}

	job.backend = self
	return job, nil
}

func (self *dbBackend) reserve(w *worker) (*Job, error) {
	var buffer bytes.Buffer

	//buffer.WriteString("SELECT id, priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, failed_at, locked_by, created_at, updated_at FROM "+ *table_name+"")
	//buffer.WriteString(select_sql_string)
	if self.isNumericParams {
		if self.dbType == POSTGRESQL || self.dbType == KINGBASE {
			buffer.WriteString(" WHERE ((run_at IS NULL OR run_at <= $3) AND (locked_at IS NULL OR locked_at < $4) OR locked_by = $5) AND failed_at IS NULL")
		} else {
			buffer.WriteString(" WHERE ((run_at IS NULL OR run_at <= $1) AND (locked_at IS NULL OR locked_at < $2) OR locked_by = $3) AND failed_at IS NULL")
		}
	} else {
		buffer.WriteString(" WHERE ((run_at IS NULL OR run_at <= ?) AND (locked_at IS NULL OR locked_at < ?) OR locked_by = ?) AND failed_at IS NULL")
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

	// Optimizations for faster lookups on some common databases
	switch self.dbType {
	case POSTGRESQL, KINGBASE:
		sqlStr := "UPDATE " + *table_name + " SET locked_at = $1, locked_by = $2 WHERE id in (SELECT id FROM " + *table_name +
			buffer.String() + " LIMIT 1) RETURNING " + fields_sql_string
		// fmt.Println(sqlStr, now, w.name, now, now.Truncate(w.max_run_time), w.name)
		rows, e := self.db.Query(sqlStr, now, w.name, now, now.Truncate(w.max_run_time), w.name)
		if nil != e {
			if sql.ErrNoRows == e {
				return nil, nil
			}
			return nil, errors.New("execute query sql failed while fetch job from the database, " + i18nString(self.dbType, self.drv, e))
		}
		defer rows.Close()

		for rows.Next() {
			return self.readJobFromRow(rows)
		}
		return nil, nil
	default:
		fmt.Println(buffer.String(), ",", now, now.Truncate(w.max_run_time), w.name)
		rows, e := self.db.Query(select_sql_string+buffer.String(), now, now.Truncate(w.max_run_time), w.name)
		if nil != e {
			if sql.ErrNoRows == e {
				return nil, nil
			}
			return nil, errors.New("execute query sql failed while fetch job from the database, " + i18nString(self.dbType, self.drv, e))
		}
		defer rows.Close()

		for rows.Next() {
			job, e := self.readJobFromRow(rows)
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
				result, e = self.db.Exec("UPDATE "+*table_name+" SET locked_at = $1, locked_by = $2 WHERE id = $3 AND (locked_at IS NULL OR locked_at < $4 OR locked_by = $5) AND failed_at IS NULL", now, w.name, job.id, now.Truncate(w.max_run_time), w.name)
			} else {
				result, e = self.db.Exec("UPDATE "+*table_name+" SET locked_at = ?, locked_by = ? WHERE id = ? AND (locked_at IS NULL OR locked_at < ? OR locked_by = ?) AND failed_at IS NULL", now, w.name, job.id, now.Truncate(w.max_run_time), w.name)
				// fmt.Println("UPDATE "+*table_name+" SET locked_at = ?, locked_by = ? WHERE id = ? AND (locked_at IS NULL OR locked_at < ? OR locked_by = ?) AND failed_at IS NULL", now, w.name, job.id, now.Truncate(w.max_run_time), w.name)
			}
			if nil != e {
				return nil, errors.New("lock job failed from the database, " + i18nString(self.dbType, self.drv, e))
			}

			c, e = result.RowsAffected()
			if nil != e {
				return nil, errors.New("lock job failed from the database, " + i18nString(self.dbType, self.drv, e))
			}

			if c > 0 {
				return job, nil
			}
		}

		e = rows.Err()
		if nil != e {
			return nil, errors.New("next job failed from the database, " + i18nString(self.dbType, self.drv, e))
		}

		return nil, nil
	}

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
	//   reserved          = self.find_by_sql(["UPDATE "+ *table_name+" SET locked_at = ?, locked_by = ? WHERE id IN (select id from "+ *table_name+" " + buffer.+") RETURNING *", now, worker.name])
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
	//   sql = ["UPDATE "+ *table_name+" SET locked_at = ?, locked_by = ? WHERE id IN (//{subquery_sql})", now, worker.name]
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
	switch self.dbType {
	case MSSQL:
		return time.Now() //.UTC()
	case POSTGRESQL, KINGBASE:
		return time.Now()
	}
	return time.Now().UTC()
}

func (self *dbBackend) create(jobs ...*Job) (e error) {
	now := self.db_time_now()

	tx, e := self.db.Begin()
	if nil != e {
		return errors.New("open transaction failed, " + i18nString(self.dbType, self.drv, e))
	}
	isCommited := false
	defer func() {
		if !isCommited {
			err := tx.Rollback()
			if nil == e {
				e = errors.New("rollback transaction failed, " + i18nString(self.dbType, self.drv, err))
			}
		}
	}()

	for _, job := range jobs {
		if job.run_at.IsZero() {
			job.run_at = now.Truncate(10 * time.Second)
		}

		// var queue sql.NullString
		// if 0 == len(job.queue) {
		// 	queue.Valid = false
		// } else {
		// 	queue.Valid = true
		// 	queue.String = job.queue
		// }

		//1         2         3      4        5           NULL        6       NULL       NULL       NULL       7           8
		//priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at
		switch self.dbType {
		case ORACLE, DM:
			_, e = tx.Exec("DELETE FROM "+*table_name+" WHERE handler_id = ?", job.handler_id)
			if nil != e {
				break
			}

			// _, e = tx.Exec("INSERT INTO "+*table_name+"(priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES (:1, :2, :3, :4, :5, NULL, :6, NULL, NULL, NULL, :7, :8)",
			// 	job.priority, job.attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
			// fmt.Println("INSERT INTO "+*table_name+"(priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES (:1, :2, :3, :4, :5, NULL, :6, NULL, NULL, NULL, :7, :8)",
			// 	job.priority, job.attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
			now_str := now.Format("2006-01-02 15:04:05")
			_, e = tx.Exec(fmt.Sprintf("INSERT INTO "+*table_name+"(priority, repeat_count, repeat_interval, attempts, max_attempts, queue, handler, handler_id, run_at, created_at, updated_at) VALUES (%d, %d, '%d', %d, %d,'%s', :1, '%s', TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'), TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'), TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'))",
				job.priority, job.repeat_count, job.repeat_interval, job.attempts, job.max_attempts, job.queue, job.handler_id, job.run_at.Format("2006-01-02 15:04:05"), now_str, now_str), job.handler)
			//fmt.Println(fmt.Sprintf("INSERT INTO "+*table_name+"(priority, attempts, queue, handler, handler_id, run_at, created_at, updated_at) VALUES (%d, %d, '%s', :1, '%s', TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'), TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'), TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS'))",
			//	job.priority, job.attempts, job.queue, job.handler_id, job.run_at.Format("2006-01-02 15:04:05"), now_str, now_str), job.handler)
		case POSTGRESQL, KINGBASE:
			_, e = tx.Exec("DELETE FROM "+*table_name+" WHERE handler_id = $1", job.handler_id)
			if nil != e {
				break
			}

			_, e = tx.Exec("INSERT INTO "+*table_name+"(priority, repeat_count, repeat_interval, attempts, max_attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, $9, NULL, NULL, NULL, $10, $11)",
				job.priority, job.repeat_count, job.repeat_interval, job.attempts, job.max_attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
			// fmt.Println("INSERT INTO "+*table_name+"(priority, repeat_count, repeat_interval, attempts, max_attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, $9, NULL, NULL, NULL, $10, $11)",
			//	job.priority, job.repeat_count, job.repeat_interval, job.attempts, job.max_attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
		default:
			_, e = tx.Exec("DELETE FROM "+*table_name+" WHERE handler_id = ?", job.handler_id)
			if nil != e {
				break
			}

			_, e = tx.Exec("INSERT INTO "+*table_name+"(priority, repeat_count, repeat_interval, attempts, max_attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL, NULL, NULL, ?, ?)",
				job.priority, job.repeat_count, job.repeat_interval, job.attempts, job.max_attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
			//fmt.Println("INSERT INTO "+*table_name+"(priority, attempts, queue, handler, handler_id, last_error, run_at, locked_at, locked_by, failed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, NULL, ?, NULL, NULL, NULL, ?, ?)",
			//	job.priority, job.attempts, job.queue, job.handler, job.handler_id, job.run_at, now, now)
		}
		if nil != e {
			return i18n(self.dbType, self.drv, e)
		}
	}

	isCommited = true
	e = tx.Commit()
	if nil != e {
		return errors.New("commit transaction failed, " + i18nString(self.dbType, self.drv, e))
	}
	return nil
}

func (self *dbBackend) update(id int64, attributes map[string]interface{}) error {
	var buffer bytes.Buffer
	params := make([]interface{}, 0, len(attributes))

	buffer.WriteString("UPDATE ")
	buffer.WriteString(*table_name)
	buffer.WriteString(" SET ")
	is_first := true

	for k, v := range attributes {
		if '@' != k[0] {
			continue
		}

		if is_first {
			is_first = false
		} else {
			buffer.WriteString(", ")
		}
		buffer.WriteString(k[1:])

		if nil == v {
			buffer.WriteString(" = NULL")
		} else {
			switch self.dbType {
			case ORACLE, DM:
				buffer.WriteString(" = :")
				buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
			case POSTGRESQL, KINGBASE:
				buffer.WriteString(" = $")
				buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
			default:
				buffer.WriteString(" = ?")
			}

			params = append(params, v)
		}
	}

	if is_first {
		is_first = false
	} else {
		buffer.WriteString(", ")
	}

	switch self.dbType {
	case ORACLE, DM:
		buffer.WriteString("updated_at = :")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		params = append(params, self.db_time_now())
		buffer.WriteString(" WHERE id = :")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		params = append(params, id)
	case POSTGRESQL, KINGBASE:
		buffer.WriteString("updated_at = $")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		params = append(params, self.db_time_now())
		buffer.WriteString(" WHERE id = $")
		buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		params = append(params, id)
	default:
		buffer.WriteString("updated_at = ?")
		params = append(params, self.db_time_now())
		buffer.WriteString(" WHERE id = ?")
		params = append(params, id)
	}

	//fmt.Println(buffer.String(), "\r\n", params)
	_, e := self.db.Exec(buffer.String(), params...)
	if nil != e && sql.ErrNoRows != e {
		return i18n(self.dbType, self.drv, e)
	}
	return nil
}

func (self *dbBackend) destroy(id int64) error {
	var e error
	if self.isNumericParams {
		_, e = self.db.Exec("DELETE FROM "+*table_name+" WHERE id = $1", id)
	} else {
		_, e = self.db.Exec("DELETE FROM "+*table_name+" WHERE id = ?", id)
	}

	if nil != e && sql.ErrNoRows != e {
		return i18n(self.dbType, self.drv, e)
	}
	return nil
}

func buildSQL(dbType int, params map[string]interface{}) (string, []interface{}, error) {
	if nil == params || 0 == len(params) {
		return "", []interface{}{}, nil
	}

	buffer := bytes.NewBuffer(make([]byte, 0, 900))
	arguments := make([]interface{}, 0, len(params))
	is_first := true
	for k, v := range params {
		if '@' != k[0] {
			continue
		}
		if is_first {
			is_first = false
			buffer.WriteString(" WHERE ")
		} else if 0 != len(arguments) {
			buffer.WriteString(" AND ")
		}

		buffer.WriteString(k[1:])
		if nil == v {
			buffer.WriteString(" IS NULL")
			continue
		}

		if "[notnull]" == v {
			buffer.WriteString(" IS NOT NULL")
			continue
		}

		switch dbType {
		case ORACLE, DM:
			buffer.WriteString(" = :")
			buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		case POSTGRESQL, KINGBASE:
			buffer.WriteString(" = $")
			buffer.WriteString(strconv.FormatInt(int64(len(params)+1), 10))
		default:
			buffer.WriteString(" = ? ")
		}

		arguments = append(arguments, v)
	}

	if groupBy, ok := params["group_by"]; ok {
		if nil == groupBy {
			return "", nil, errors.New("groupBy is empty.")
		}

		s := fmt.Sprint(groupBy)
		if 0 == len(s) {
			return "", nil, errors.New("groupBy is empty.")
		}

		buffer.WriteString(" GROUP BY ")
		buffer.WriteString(s)
	}

	if having_v, ok := params["having"]; ok {
		if nil == having_v {
			return "", nil, errors.New("having is empty.")
		}

		having := fmt.Sprint(having_v)
		if 0 == len(having) {
			return "", nil, errors.New("having is empty.")
		}

		buffer.WriteString(" HAVING ")
		buffer.WriteString(having)
	}

	if order_v, ok := params["order_by"]; ok {
		if nil == order_v {
			return "", nil, errors.New("order is empty.")
		}

		order := fmt.Sprint(order_v)
		if 0 == len(order) {
			return "", nil, errors.New("order is empty.")
		}

		buffer.WriteString(" ORDER BY ")
		buffer.WriteString(order)
	}

	if limit_v, ok := params["limit"]; ok {
		if nil == limit_v {
			return "", nil, errors.New("limit is not a number, actual value is nil")
		}
		limit := fmt.Sprint(limit_v)
		i, e := strconv.ParseInt(limit, 10, 64)
		if nil != e {
			return "", nil, fmt.Errorf("limit is not a number, actual value is '" + limit + "'")
		}
		if i <= 0 {
			return "", nil, fmt.Errorf("limit must is geater zero, actual value is '" + limit + "'")
		}

		if offset_v, ok := params["offset"]; ok {
			if nil == offset_v {
				return "", nil, errors.New("offset is not a number, actual value is nil")
			}
			offset := fmt.Sprint(offset_v)
			i, e = strconv.ParseInt(offset, 10, 64)
			if nil != e {
				return "", nil, fmt.Errorf("offset is not a number, actual value is '" + offset + "'")
			}

			if i < 0 {
				return "", nil, fmt.Errorf("offset must is geater(or equals) zero, actual value is '" + offset + "'")
			}

			buffer.WriteString(" LIMIT ")
			buffer.WriteString(offset)
			buffer.WriteString(" , ")
			buffer.WriteString(limit)
		} else {
			buffer.WriteString(" LIMIT ")
			buffer.WriteString(limit)
		}
	}

	return buffer.String(), arguments, nil
}

func (self *dbBackend) count(params map[string]interface{}) (int64, error) {
	query, arguments, e := buildSQL(self.dbType, params)
	if nil != e {
		return 0, e
	}

	count := int64(0)
	e = self.db.QueryRow("SELECT count(*) FROM "+*table_name+query, arguments...).Scan(&count)
	if nil != e {
		if sql.ErrNoRows == e {
			return 0, nil
		}
		return 0, i18n(self.dbType, self.drv, e)
	}
	return count, nil
}

func (self *dbBackend) where(params map[string]interface{}) ([]map[string]interface{}, error) {
	query, arguments, e := buildSQL(self.dbType, params)
	if nil != e {
		return nil, i18n(self.dbType, self.drv, e)
	}

	//// fmt.Println(select_sql_string + query)
	rows, e := self.db.Query(select_sql_string+query, arguments...)
	if nil != e {
		if sql.ErrNoRows == e {
			return nil, nil
		}
		return nil, i18n(self.dbType, self.drv, e)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var priority int
		var repeat_count int
		var repeat_interval sql.NullString
		var attempts int
		var max_attempts int
		var handler string
		var handler_id sql.NullString
		var created_at time.Time
		var updated_at time.Time

		var queue sql.NullString
		var last_error sql.NullString
		var run_at NullTime
		var locked_at NullTime
		var failed_at NullTime
		var locked_by sql.NullString

		e = rows.Scan(
			&id,
			&priority,
			&repeat_count,
			&repeat_interval,
			&attempts,
			&max_attempts,
			&queue,
			&handler,
			&handler_id,
			&last_error,
			&run_at,
			&locked_at,
			&failed_at,
			&locked_by,
			&created_at,
			&updated_at)
		if nil != e {
			return nil, i18n(self.dbType, self.drv, e)
		}

		result := map[string]interface{}{"id": id,
			"priority":     priority,
			"repeat_count": repeat_count,
			"attempts":     attempts,
			"max_attempts": max_attempts,
			"handler":      handler,
			"handler_id":   handler_id,
			"created_at":   created_at,
			"updated_at":   updated_at}

		// var queue sql.NullString
		// var last_error sql.NullString
		// var run_at NullTime
		// var locked_at NullTime
		// var failed_at NullTime
		// var locked_by sql.NullString

		if queue.Valid {
			result["queue"] = queue.String
		}
		if repeat_interval.Valid {
			result["repeat_interval"] = repeat_interval.String
		}

		if last_error.Valid {
			result["last_error"] = last_error.String
			if 20 < len(last_error.String) {
				result["last_error_summary"] = last_error.String[0:20] + "..."
			} else {
				result["last_error_summary"] = last_error.String
			}
		}

		if run_at.Valid {
			result["run_at"] = run_at.Time
		}

		if locked_at.Valid {
			result["locked_at"] = locked_at.Time
		}

		if failed_at.Valid {
			result["failed"] = true
			result["failed_at"] = failed_at.Time
		} else {
			result["failed"] = false
		}

		if locked_by.Valid {
			result["locked_by"] = locked_by.String
		}

		results = append(results, result)
	}

	e = rows.Err()
	if nil != e {
		return nil, i18n(self.dbType, self.drv, e)
	}
	return results, nil
}

// func (self *dbBackend) all() ([]map[string]interface{}, error) {
// 	return self.where("")
// }

// func (self *dbBackend) failed() ([]map[string]interface{}, error) {
// 	return self.where("failed_at IS NOT NULL")
// }

// func (self *dbBackend) active() ([]map[string]interface{}, error) {
// 	return self.where("failed_at IS NULL AND locked_by IS NOT NULL")
// }

// func (self *dbBackend) queued() ([]map[string]interface{}, error) {
// 	return self.where("failed_at IS NULL AND locked_by IS NULL")
// }

func (self *dbBackend) retry(id int64) error {
	return self.update(id, map[string]interface{}{"@failed_at": nil})
}
