package delayed_job

import (
	"encoding/json"
	"errors"
	_ "expvar"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/rakyll/statik/fs"

	_ "github.com/runner-mei/delayed_job/statik"
)

var (
	config_file = flag.String("delayed-config", "", "the config file name")
	cd_dir, _   = os.Getwd()

	retry_list = []*regexp.Regexp{regexp.MustCompile(`^/?[0-9]+/retry/?$`),
		regexp.MustCompile(`^/?delayed_jobs/[0-9]+/retry/?$`),
		regexp.MustCompile(`^/?delayed_jobs/delayed_jobs/[0-9]+/retry/?$`)}

	delete_by_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[0-9]+/delete/?$`),
		regexp.MustCompile(`^/?delayed_jobs/[0-9]+/delete/?$`),
		regexp.MustCompile(`^/?delayed_jobs/delayed_jobs/[0-9]+/delete/?$`)}

	job_id_list = []*regexp.Regexp{regexp.MustCompile(`^/?[0-9]+/?$`),
		regexp.MustCompile(`^/?delayed_jobs/[0-9]+/?$`),
		regexp.MustCompile(`^/?delayed_jobs/delayed_jobs/[0-9]+/?$`)}
)

func abs(pa string) string {
	s, e := filepath.Abs(pa)
	if nil != e {
		panic(e.Error())
	}
	return s
}

func searchFile() (string, bool) {
	files := []string{*config_file,
		filepath.Join("data", "conf", "delayed_job.conf"),
		filepath.Join("data", "etc", "delayed_job.conf"),
		filepath.Join("..", "data", "conf", "delayed_job.conf"),
		filepath.Join("..", "data", "etc", "delayed_job.conf"),
		"/etc/tpt/delayed_job.conf"}

	for _, file := range files {
		if st, e := os.Stat(file); nil == e && nil != st && !st.IsDir() {
			return abs(file), true
		}
	}

	files = []string{filepath.Join("data", "conf"),
		filepath.Join("data", "etc"),
		filepath.Join("..", "data", "conf"),
		filepath.Join("..", "data", "etc"),
		"/etc/tpt"}
	for _, file := range files {
		if st, e := os.Stat(file); nil == e && nil != st && st.IsDir() {
			return abs(filepath.Join(file, "delayed_job.conf")), false
		}
	}
	if runtime.GOOS == "windows" {
		return abs(filepath.Join("data/conf/delayed_job.conf")), false
	} else {
		return abs(filepath.Join("/etc/tpt/delayed_job.conf")), false
	}
}

func Main(runMode, dbDrv, dbURL string, runHttp func(http.Handler)) error {
	default_actuals = loadActualFlags(nil)
	initDB()

	if *config_file == "" {
		file, found := searchFile()
		flag.Set("delayed-config", file)
		fmt.Println("[info] config file is '" + file + "'")

		if found {
			fmt.Println("[warn] load file '" + file + "'.")
			e := loadConfig(file, nil, false)
			if nil != e {
				return errors.New("load file '" + file + "' failed, " + e.Error())
			}
		}
	}

	fmt.Println("useTLS=", *default_mail_useTLS)
	fmt.Println("useFQDN=", *default_mail_useFQDN)

	if !fileExists(gammu_config) {
		for _, s := range []string{"data/conf/gammu.conf",
			"data/etc/gammu.conf",
			"../data/conf/gammu.conf",
			"../data/etc/gammu.conf",
			"/etc/tpt/gammu.conf"} {
			if fileExists(s) {
				flag.Set("gammu_config", s)
				break
			}
		}
	}

	switch runMode {
	case "init_db":
		ctx := map[string]interface{}{}
		backend, e := newBackend(dbDrv, dbURL, ctx)
		if nil != e {
			return e
		}
		defer backend.Close()
		switch ToDbType(dbDrv) {
		case MSSQL:
			script := `if object_id('dbo.` + *table_name + `', 'U') is not null
				BEGIN 
							 DROP TABLE ` + *table_name + `; 
				END
				if object_id('dbo.` + *table_name + `', 'U') is null
				BEGIN
				 CREATE TABLE dbo.` + *table_name + ` (
						  id                INT IDENTITY(1,1)  PRIMARY KEY,
						  priority          int DEFAULT 0,
						  repeat_count      int DEFAULT 0,
						  repeat_interval   varchar(20) DEFAULT '',
						  attempts          int DEFAULT 0,
						  max_attempts      int DEFAULT 0,
						  queue             varchar(200),
						  handler           text  NOT NULL,
						  handler_id        varchar(200),
						  last_error        varchar(2000),
						  run_at            DATETIME2,
						  locked_at         DATETIME2,
						  failed_at         DATETIME2,
						  locked_by         varchar(200),
						  created_at        DATETIME2 NOT NULL,
						  updated_at        DATETIME2 NOT NULL
						); 
				END`
			fmt.Println(script)
			_, e = backend.db.Exec(script)
			if nil != e {
				return e
			}
		case POSTGRESQL:
			script := `DROP TABLE IF EXISTS ` + *table_name + `;
				CREATE TABLE IF NOT EXISTS ` + *table_name + ` (
				  id                SERIAL PRIMARY KEY,
				  priority          int DEFAULT 0,
		      repeat_count      int DEFAULT 0,
		      repeat_interval   varchar(20) DEFAULT '',
				  attempts          int DEFAULT 0,
		      max_attempts      int DEFAULT 0,
				  queue             varchar(200),
				  handler           text  NOT NULL,
				  handler_id        varchar(200),
				  last_error        varchar(2000),
				  run_at            timestamp with time zone,
				  locked_at         timestamp with time zone,
				  failed_at         timestamp with time zone,
				  locked_by         varchar(200),
				  created_at        timestamp with time zone NOT NULL,
				  updated_at        timestamp with time zone NOT NULL
				);`
			fmt.Println(script)
			_, e = backend.db.Exec(script)
			if nil != e {
				return e
			}
		case ORACLE, DM:
			for _, script := range []string{
				`DROP TABLE IF EXISTS ` + *table_name,
				`CREATE TABLE ` + *table_name + ` (
					  id                INT IDENTITY(1,1)  PRIMARY KEY,
					  priority          NUMBER(10) DEFAULT 0,
					  repeat_count      NUMBER(10) DEFAULT 0,
					  repeat_interval   varchar2(20) DEFAULT '',
					  attempts          NUMBER(10) DEFAULT 0,
					  max_attempts      NUMBER(10) DEFAULT 0,
					  queue             varchar2(200 BYTE),
					  handler           clob,--  NOT NULL,
					  handler_id        varchar2(200 BYTE),
					  last_error        VARCHAR2(2000 BYTE),
					  run_at            timestamp with time zone,
					  locked_at         timestamp with time zone,
					  failed_at         timestamp with time zone,
					  locked_by         varchar2(200 BYTE),
					  created_at        timestamp with time zone, -- NOT NULL,
					  updated_at        timestamp with time zone
					)`,
			} {
				fmt.Println(script)
				_, e = backend.db.Exec(script)
				if nil != e {
					return i18n(ORACLE, "oci8", e)
				}
			}
		default:
			for _, script := range []string{`DROP TABLE IF EXISTS ` + *table_name + `;`,
				`CREATE TABLE IF NOT EXISTS ` + *table_name + ` (
					  id                SERIAL PRIMARY KEY,
					  priority          int DEFAULT 0,
		        repeat_count      int DEFAULT 0,
		        repeat_interval   varchar(20) DEFAULT '',
					  attempts          int DEFAULT 0,
		        max_attempts      int DEFAULT 0,
					  queue             varchar(200),
					  handler           text  NOT NULL,
					  handler_id        varchar(200),
					  last_error        VARCHAR(2000),
					  run_at            DATETIME,
					  locked_at         DATETIME,
					  failed_at         DATETIME,
					  locked_by         varchar(200),
					  created_at        DATETIME NOT NULL,
					  updated_at        timestamp NOT NULL
					);`} {
				fmt.Println(script)
				_, e = backend.db.Exec(script)
				if nil != e {
					return e
				}
			}
		}

	case "console":
		ctx := map[string]interface{}{}
		backend, e := newBackend(dbDrv, dbURL, ctx)
		if nil != e {
			return e
		}

		nm := filepath.Base(os.Args[0])
		if !isPidInitialize() {
			if "windows" == runtime.GOOS {
				flag.Set("pid_file", nm+".pid")
			} else {
				flag.Set("pid_file", "/var/run/tpt/"+nm+".pid")
			}
		}
		if err := createPidFile(*pidFile, nm); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer removePidFile(*pidFile)
		httpServe(backend, findFs(), runHttp)
	case "backend":
		w, e := newWorker(map[string]interface{}{
			"db_drv": dbDrv,
			"db_url": dbURL,
		})
		if nil != e {
			return e
		}

		nm := filepath.Base(os.Args[0])
		if !isPidInitialize() {
			if "windows" == runtime.GOOS {
				flag.Set("pid_file", nm+".pid")
			} else {
				flag.Set("pid_file", "/var/run/tpt/"+nm+".pid")
			}
		}
		if err := createPidFile(*pidFile, nm); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer removePidFile(*pidFile)

		w.RunForever()
	case "all":
		w, e := newWorker(map[string]interface{}{
			"db_drv": dbDrv,
			"db_url": dbURL,
		})
		if nil != e {
			return e
		}

		nm := filepath.Base(os.Args[0])
		if !isPidInitialize() {
			if "windows" == runtime.GOOS {
				flag.Set("pid_file", nm+".pid")
			} else {
				flag.Set("pid_file", "/var/run/tpt/"+nm+".pid")
			}
		}
		if err := createPidFile(*pidFile, nm); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer removePidFile(*pidFile)
		go httpServe(w.backend, findFs(), runHttp)
		w.RunForever()
	}
	return nil
}

func findFs() http.Handler {
	for _, s := range []string{filepath.Join(".", "index.html"),
		filepath.Join("public", "index.html"),
		filepath.Join("..", "public", "index.html"),
		filepath.Join("lib", "delayed_jobs", "index.html")} {
		if fileExists(s) {
			log.Println("public directory is found in the", s)
			return http.FileServer(http.Dir(filepath.Dir(s)))
		}
	}
	return nil
}

func fileExists(nm string) bool {
	fs, e := os.Stat(nm)
	if nil != e {
		return false
	}
	return !fs.IsDir()
}

func fileHandler(w http.ResponseWriter, r *http.Request, path, default_content string) {
	var name string
	if filepath.IsAbs(path) {
		name = path
	} else {
		name = cd_dir + path
	}
	if fileExists(name) {
		http.ServeFile(w, r, name)
		return
	}

	io.WriteString(w, default_content)
}

// func indexHandlerWithMessage(w http.ResponseWriter, r *http.Request, level, message string) {
// 	io.WriteString(w, index_html_1)

// 	io.WriteString(w, "<div class=\"alert alert-")
// 	io.WriteString(w, level)
// 	io.WriteString(w, "\"> ")
// 	io.WriteString(w, html.EscapeString(message))
// 	io.WriteString(w, " </div>")

// 	io.WriteString(w, index_html_2)
// }

// func (self *dbBackend) all() ([]map[string]interface{}, error) {
// 	return self.where("")
// }

// func (self *dbBackend) failed() ([]map[string]interface{}, error) {
// 	return self.where("failed_at IS NOT NULL")
// }

// func (self *dbBackend) active() ([]map[string]interface{}, error) {index_html_1
// 	return self.where("failed_at IS NULL AND locked_by IS NOT NULL")
// }

// func (self *dbBackend) queued() ([]map[string]interface{}, error) {
// 	return self.where("failed_at IS NULL AND locked_by IS NULL")
// }

// func (self *dbBackend) retry(id int) error {
// 	return self.backend.update(id, map[string]interface{}{"@failed_at": nil})
// }

func queryHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend, params map[string]interface{}) {
	results, e := backend.where(params)
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}

	w.Header()["Content-Type"] = []string{"application/json; charset=utf-8"}
	e = json.NewEncoder(w).Encode(results)
	if nil != e {
		w.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}
}

func allHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	queryHandler(w, r, backend, nil)
}

func failedHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	//return self.where("failed_at IS NOT NULL")
	queryHandler(w, r, backend, map[string]interface{}{"@failed_at": "[notnull]"})
}

func queuedHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	queryHandler(w, r, backend, map[string]interface{}{"@failed_at": nil, "locked_by": nil})
	// 	return self.where("failed_at IS NULL AND locked_by IS NULL")
}

func activeHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	//return self.where("failed_at IS NULL AND locked_by IS NOT NULL")
	queryHandler(w, r, backend, map[string]interface{}{"@failed_at": nil, "locked_by": "[notnull]"})
}

func countsHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	var all_size, failed_size, queued_size, active_size int64
	var e error

	all_size, e = backend.count(nil)
	if nil != e {
		goto failed
	}
	failed_size, e = backend.count(map[string]interface{}{"@failed_at": "[notnull]"})
	if nil != e {
		goto failed
	}
	queued_size, e = backend.count(map[string]interface{}{"@failed_at": nil, "locked_by": nil})
	if nil != e {
		goto failed
	}
	active_size, e = backend.count(map[string]interface{}{"@failed_at": nil, "locked_by": "[notnull]"})
	if nil != e {
		goto failed
	}

	w.Header()["Content-Type"] = []string{"application/json; charset=utf-8"}
	io.WriteString(w, fmt.Sprintf(`{"all":%v,"failed":%v,"active":%v,"queued":%v}`, all_size, failed_size, active_size, queued_size))
	return

failed:
	w.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
	w.WriteHeader(http.StatusInternalServerError)
	io.WriteString(w, e.Error())
	return
}

func testJobHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var ent map[string]interface{}
	e := decoder.Decode(&ent)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}


	handler_o, ok := ent["handler"]
	if ok {
		handler, ok := handler_o.(map[string]interface{})
		if ok {
			if _, ok := handler["content"]; !ok {
				handler["content"] = "this is test job message. 这是一个测试消息。"
			}
		}
	}


	job, e := createJobFromMap(backend, ent)
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}

	e = job.invokeJob()
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
	return
}

func pushHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var ent map[string]interface{}
	e := decoder.Decode(&ent)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}

	job, e := createJobFromMap(backend, ent)
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}

	e = backend.create(job)
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
	return
}

func pushAllHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	var jobs []*Job
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var entities []map[string]interface{}
	e := decoder.Decode(&entities)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}
	if nil == entities || 0 == len(entities) {
		goto OK
	}

	jobs = make([]*Job, len(entities))
	for i, ent := range entities {
		jobs[i], e = createJobFromMap(backend, ent)
		if nil != e {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "parse data["+strconv.FormatInt(int64(i), 10)+"] failed, "+e.Error())
			return
		}
	}

	backend.create(jobs...)
	if nil != e {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, e.Error())
		return
	}
OK:
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
	return
}

func readSettingsFileHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	fileHandler(w, r, *config_file, "{}")
}

func settingsFileHandler(w http.ResponseWriter, r *http.Request, backend *dbBackend) {
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var entities map[string]interface{}
	e := decoder.Decode(&entities)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}

	e = assignFlagSet("", entities, nil, default_actuals, false)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}

	f, e := os.OpenFile(*config_file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if nil != e {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}
	defer f.Close()

	b, e := json.MarshalIndent(entities, "", "  ")
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}
	_, e = f.Write(b)
	if e != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, e.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
	return
}

type webFront struct {
	fs http.Handler
	*dbBackend
}

func (self *webFront) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend := self.dbBackend

	switch r.Method {
	case "GET":
		switch r.URL.Path {
		case "/all":
			allHandler(w, r, backend)
			return
		case "/failed":
			failedHandler(w, r, backend)
			return
		case "/queued":
			queuedHandler(w, r, backend)
			return
		case "/active":
			activeHandler(w, r, backend)
			return
		case "/counts":
			countsHandler(w, r, backend)
			return
		case "/settings_file", "/delayed_jobs/settings_file", "/delayed_job/settings_file":
			readSettingsFileHandler(w, r, backend)
			return
		default:
			if !strings.HasPrefix(r.URL.Path, "/debug/") {
				if nil == self.fs {
					statikFS, err := fs.New()
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						io.WriteString(w, err.Error())
						return
					}
					//http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(statikFS)))
					self.fs = http.StripPrefix("/", http.FileServer(statikFS))
				}
				self.fs.ServeHTTP(w, r)
				return
			}
		}

	case "PUT":
		switch r.URL.Path {
		case "/test", "/delayed_jobs/test", "/delayed_job/test":
			testJobHandler(w, r, backend)
			return

		case "/push":
			pushHandler(w, r, backend)
			return

		case "/pushAll":
			pushAllHandler(w, r, backend)
			return

		case "/settings_file", "/delayed_jobs/settings_file", "/delayed_job/settings_file":
			settingsFileHandler(w, r, backend)
			return
		}

	case "POST":
		switch r.URL.Path {
		case "/test", "/delayed_jobs/test":
			testJobHandler(w, r, backend)
			return

		case "/push":
			pushHandler(w, r, backend)
			return

		case "/pushAll":
			pushAllHandler(w, r, backend)
			return

		case "/settings_file", "/delayed_jobs/settings_file", "/delayed_job/settings_file":
			settingsFileHandler(w, r, backend)
			return
		}

		for _, retry := range retry_list {
			if retry.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
				if nil != e {
					w.WriteHeader(http.StatusBadRequest)
					io.WriteString(w, e.Error())
					return
				}

				e = backend.retry(id)
				if nil == e {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "The job has been queued for a re-run")
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				}
				return
			}
		}

		for _, job_id := range delete_by_id_list {
			if job_id.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
				if nil != e {
					w.WriteHeader(http.StatusBadRequest)
					io.WriteString(w, e.Error())
					return
				}

				e = backend.destroy(id)
				if nil == e {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "The job was deleted")
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				}
				return
			}
		}
	case "DELETE":
		for _, job_id := range job_id_list {
			if job_id.MatchString(r.URL.Path) {
				ss := strings.Split(r.URL.Path, "/")
				id, e := strconv.ParseInt(ss[len(ss)-1], 10, 0)
				if nil != e {
					w.WriteHeader(http.StatusBadRequest)
					io.WriteString(w, e.Error())
					return
				}

				e = backend.destroy(id)
				if nil == e {
					w.WriteHeader(http.StatusOK)
					io.WriteString(w, "The job was deleted")
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					io.WriteString(w, e.Error())
				}
				return
			}
		}
	}

	http.DefaultServeMux.ServeHTTP(w, r)
}

func httpServe(backend *dbBackend, handler http.Handler, runHttp func(http.Handler)) {
	runHttp(&webFront{dbBackend: backend, fs: handler})
}
