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
	WorkTest(t, GetTestConnDrv(), GetTestConnURL(), func(w *TestWorker) {
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
		e := backend.enqueue(1, 0, "", 1, "aa", time.Time{}, map[string]interface{}{"type": "test"})
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

func TestRunErrorAndRescheduleIt(t *testing.T) {
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 0, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a"})
		if nil != e {
			t.Error(e)
			return
		}

		select {
		case <-test_chan:
		case <-time.After(2 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(500 * time.Millisecond)

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM " + *table_name)

		var attempts int64
		var run_at NullTime
		var locked_at NullTime
		var locked_by sql.NullString
		var handler NullString
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

		if !strings.Contains(handler.String, "UpdatePayloadObject") {
			t.Error("excepted handler contains 'UpdatePayloadObject', actual is ", handler.String)
		}

		if "throw a" != last_error.String {
			t.Error("excepted run_at is 'throw a', actual is", last_error.String)
		}

	})
}

func TestRunFailedAndNotDestoryIt2(t *testing.T) {
	*default_destroy_failed_jobs = false
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a"})
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
		e := backend.enqueue(1, 0, "", 1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a"})
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
			var handler NullString
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
			interval := now.Sub(run_at.Time)
			if interval < 0 {
				interval = -interval
			}
			if interval > 1*time.Second {
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
		e := backend.enqueue(1, 0, "", 1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": "throw a"})
		if nil != e {
			t.Error(e)
			return
		}

		select {
		case <-test_chan:
		case <-time.After(2 * time.Second):
			t.Error("not recv")
		}

		var count int64
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)

			e = backend.db.QueryRow("SELECT count(*) FROM " + *table_name).Scan(&count)
			if nil != e {
				t.Error(e)
				return
			}
			if count == 0 {
				break
			}
		}

		if 0 != count {
			t.Error("excepted jobs is empty, actual is", count)
		}
	})
}

var max_message_txt = `java.sql.SQLIntegrityConstraintViolationException: ORA-01400: 无法将 NULL 插入 ("GLASMS"."SM_SEND_SM_LIST"."SMCONTENT")

	at oracle.jdbc.driver.T4CTTIoer.processError(T4CTTIoer.java:445)
	at oracle.jdbc.driver.T4CTTIoer.processError(T4CTTIoer.java:396)
	at oracle.jdbc.driver.T4C8Oall.processError(T4C8Oall.java:879)
	at oracle.jdbc.driver.T4CTTIfun.receive(T4CTTIfun.java:450)
	at oracle.jdbc.driver.T4CTTIfun.doRPC(T4CTTIfun.java:192)
	at oracle.jdbc.driver.T4C8Oall.doOALL(T4C8Oall.java:531)
	at oracle.jdbc.driver.T4CStatement.doOall8(T4CStatement.java:193)
	at oracle.jdbc.driver.T4CStatement.executeForRows(T4CStatement.java:1033)
	at oracle.jdbc.driver.OracleStatement.doExecuteWithTimeout(OracleStatement.java:1329)
	at oracle.jdbc.driver.OracleStatement.executeInternal(OracleStatement.java:1909)
	at oracle.jdbc.driver.OracleStatement.execute(OracleStatement.java:1871)
	at oracle.jdbc.driver.OracleStatementWrapper.execute(OracleStatementWrapper.java:318)
	at com.tpt.jbridge.core.db.DbResult.exec(DbResult.java:109)
	at com.tpt.jbridge.core.db.DBServlet.exec(DBServlet.java:57)
	at com.tpt.jbridge.core.db.DBServlet.doGet(DBServlet.java:19)
	at javax.servlet.http.HttpServlet.service(HttpServlet.java:687)
	at javax.servlet.http.HttpServlet.service(HttpServlet.java:790)
	at io.undertow.servlet.handlers.ServletHandler.handleRequest(ServletHandler.java:85)
	at io.undertow.servlet.handlers.security.ServletSecurityRoleHandler.handleRequest(ServletSecurityRoleHandler.java:61)
	at io.undertow.servlet.handlers.ServletDispatchingHandler.handleRequest(ServletDispatchingHandler.java:36)
	at io.undertow.servlet.handlers.security.SSLInformationAssociationHandler.handleRequest(SSLInformationAssociationHandler.java:131)
	at io.undertow.servlet.handlers.security.ServletAuthenticationCallHandler.handleRequest(ServletAuthenticationCallHandler.java:56)
	at io.undertow.server.handlers.PredicateHandler.handleRequest(PredicateHandler.java:43)
	at io.undertow.security.handlers.AbstractConfidentialityHandler.handleRequest(AbstractConfidentialityHandler.java:45)
	at io.undertow.servlet.handlers.security.ServletConfidentialityConstraintHandler.handleRequest(ServletConfidentialityConstraintHandler.java:63)
	at io.undertow.security.handlers.AuthenticationMechanismsHandler.handleRequest(AuthenticationMechanismsHandler.java:58)
	at io.undertow.servlet.handlers.security.CachedAuthenticatedSessionHandler.handleRequest(CachedAuthenticatedSessionHandler.java:70)
	at io.undertow.security.handlers.SecurityInitialHandler.handleRequest(SecurityInitialHandler.java:76)
	at io.undertow.server.handlers.PredicateHandler.handleRequest(PredicateHandler.java:43)
	at io.undertow.server.handlers.PredicateHandler.handleRequest(PredicateHandler.java:43)
	at io.undertow.servlet.handlers.ServletInitialHandler.handleFirstRequest(ServletInitialHandler.java:261)
	at io.undertow.servlet.handlers.ServletInitialHandler.dispatchRequest(ServletInitialHandler.java:247)
	at io.undertow.servlet.handlers.ServletInitialHandler.handleRequest(ServletInitialHandler.java:180)
	at io.undertow.server.handlers.HttpContinueReadHandler.handleRequest(HttpContinueReadHandler.java:64)
	at io.undertow.server.handlers.PathHandler.handleRequest(PathHandler.java:56)
	at io.undertow.server.Connectors.executeRootHandler(Connectors.java:197)
	at io.undertow.server.HttpServerExchange$1.run(HttpServerExchange.java:759)
	at java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1145)
	at java.util.concurrent.ThreadPoolExecutor$Worker.run(ThreadPoolExecutor.java:615)
	at java.lang.Thread.run(Thread.java:745)
`

func TestRunWithMaxErrorAndRescheduleIt(t *testing.T) {
	workTest(t, func(w *worker, backend *dbBackend) {
		e := backend.enqueue(1, 0, "", 1, "aa", time.Time{}, map[string]interface{}{"type": "test", "try_interval": "0s", "error": max_message_txt})
		if nil != e {
			t.Error(e)
			return
		}

		select {
		case <-test_chan:
		case <-time.After(2 * time.Second):
			t.Error("not recv")
		}
		time.Sleep(500 * time.Millisecond)

		var count int64
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)

			e = backend.db.QueryRow("SELECT count(*) FROM " + *table_name + " WHERE attempts <> 1").Scan(&count)
			if nil != e {
				t.Error(e)
				return
			}
			if count == 0 {
				break
			}
		}

		row := backend.db.QueryRow("SELECT attempts, run_at, locked_at, locked_by, handler, last_error FROM " + *table_name)

		var attempts int64
		var run_at NullTime
		var locked_at NullTime
		var locked_by sql.NullString
		var handler NullString
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

		if !strings.Contains(handler.String, "UpdatePayloadObject") {
			t.Error("excepted handler contains 'UpdatePayloadObject', actual is ", handler.String)
		}

		excepted := `java.sql.SQLIntegrityConstraintViolationException: ORA-01400`
		if !strings.Contains(last_error.String, excepted) {
			t.Error("excepted run_at is '"+excepted+"', actual is", last_error.String)
		}

	})
}
