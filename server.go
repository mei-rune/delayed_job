package delayed_job

import (
	"encoding/json"
	_ "expvar"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	listenAddress = flag.String("listen", ":9086", "the address of http")
	run_mode      = flag.String("mode", "all", "console, backend, all")

	cd_dir, _ = os.Getwd()

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

func Main() error {
	flag.Parse()
	if nil != flag.Args() && 0 != len(flag.Args()) {
		flag.Usage()
		return nil
	}

	switch *run_mode {
	case "init_db":
		ctx := map[string]interface{}{}
		backend, e := newBackend(*db_drv, *db_url, ctx)
		if nil != e {
			return e
		}
		defer backend.Close()

		_, e = backend.db.Exec(`
DROP TABLE IF EXISTS ` + *table_name + `;

CREATE TABLE IF NOT EXISTS ` + *table_name + ` (
  id                BIGSERIAL  PRIMARY KEY,
  priority          int DEFAULT 0,
  attempts          int DEFAULT 0,
  queue             varchar(200),
  handler           text  NOT NULL,
  handler_id        varchar(200),
  last_error        varchar(2000),
  run_at            timestamp with time zone,
  locked_at         timestamp with time zone,
  failed_at         timestamp with time zone,
  locked_by         varchar(200),
  created_at        timestamp with time zone  NOT NULL,
  updated_at        timestamp with time zone NOT NULL
);`)
		if nil != e {
			return e
		}

	case "console":
		ctx := map[string]interface{}{}
		backend, e := newBackend(*db_drv, *db_url, ctx)
		if nil != e {
			return e
		}
		httpServe(backend)
	case "backend":
		w, e := newWorker(map[string]interface{}{})
		if nil != e {
			return e
		}
		w.RunForever()
	case "all":
		w, e := newWorker(map[string]interface{}{})
		if nil != e {
			return e
		}
		go httpServe(w.backend)
		w.RunForever()
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
	name := cd_dir + path
	if fileExists(name) {
		http.ServeFile(w, r, name)
		return
	}

	io.WriteString(w, default_content)
}

func bootstrapCssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/css; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/bootstrap.css", bootstrap_css)
}
func bootstrapModalJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/bootstrap_modal.js", bootstrap_modal_js)
}
func bootstrapPopoverJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/bootstrap_popover.js", bootstrap_popover_js)
}
func bootstrapTabJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/bootstrap_tab.js", bootstrap_tab_js)
}
func bootstrapTooltipJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/bootstrap_tooltip.js", bootstrap_tooltip_js)
}
func djmonCssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/css; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/dj_mon.css", dj_mon_css)
}
func djmonJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/dj_mon.js", dj_mon_js)
}
func jqueryJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/jquery.min.js", jquery_min_js)
}
func mustascheJsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"text/javascript; charset=utf-8"}
	fileHandler(w, r, "/static/delayed_jobs/mustasche.js", mustasche_js)
}
func indexHandler(w http.ResponseWriter, r *http.Request) {
	fileHandler(w, r, "/index.html", index_html)
}

func indexHandlerWithMessage(w http.ResponseWriter, r *http.Request, level, message string) {
	io.WriteString(w, index_html_1)

	io.WriteString(w, "<div class=\"alert alert-")
	io.WriteString(w, level)
	io.WriteString(w, "\"> ")
	io.WriteString(w, html.EscapeString(message))
	io.WriteString(w, " </div>")

	io.WriteString(w, index_html_2)
}

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

//http://127.0.0.1:9086/delayed_jobs/1/retry

func httpServe(backend *dbBackend) {
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				switch r.URL.Path {
				case "/", "/index.html", "/index.htm", "/delayed_jobs", "/delayed_jobs/":
					indexHandler(w, r)
					return
				case "/static/delayed_jobs/bootstrap.css":
					bootstrapCssHandler(w, r)
					return
				case "/static/delayed_jobs/bootstrap_modal.js":
					bootstrapModalJsHandler(w, r)
					return
				case "/static/delayed_jobs/bootstrap_popover.js":
					bootstrapPopoverJsHandler(w, r)
					return
				case "/static/delayed_jobs/bootstrap_tab.js":
					bootstrapTabJsHandler(w, r)
					return
				case "/static/delayed_jobs/bootstrap_tooltip.js":
					bootstrapTooltipJsHandler(w, r)
					return
				case "/static/delayed_jobs/dj_mon.css":
					djmonCssHandler(w, r)
					return
				case "/static/delayed_jobs/dj_mon.js":
					djmonJsHandler(w, r)
					return
				case "/static/delayed_jobs/jquery.min.js":
					jqueryJsHandler(w, r)
					return
				case "/static/delayed_jobs/mustache.js":
					mustascheJsHandler(w, r)
					return

				case "/all", "/delayed_jobs/all", "/delayed_jobs/delayed_jobs/all":
					allHandler(w, r, backend)
					return
				case "/failed", "/delayed_jobs/failed", "/delayed_jobs/delayed_jobs/failed":
					failedHandler(w, r, backend)
					return
				case "/queued", "/delayed_jobs/queued", "/delayed_jobs/delayed_jobs/queued":
					queuedHandler(w, r, backend)
					return
				case "/active", "/delayed_jobs/active", "/delayed_jobs/delayed_jobs/active":
					activeHandler(w, r, backend)
					return
				case "/counts", "/delayed_jobs/counts", "/delayed_jobs/delayed_jobs/counts":
					countsHandler(w, r, backend)
					return
				}
			case "POST":
				for _, retry := range retry_list {
					if retry.MatchString(r.URL.Path) {
						ss := strings.Split(r.URL.Path, "/")
						id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
						if nil != e {
							indexHandlerWithMessage(w, r, "error", e.Error())
							return
						}

						e = backend.retry(id)
						if nil == e {
							indexHandlerWithMessage(w, r, "success", "The job has been queued for a re-run")
						} else {
							indexHandlerWithMessage(w, r, "error", e.Error())
						}
						return
					}
				}

				for _, job_id := range delete_by_id_list {
					if job_id.MatchString(r.URL.Path) {
						ss := strings.Split(r.URL.Path, "/")
						id, e := strconv.ParseInt(ss[len(ss)-2], 10, 0)
						if nil != e {
							indexHandlerWithMessage(w, r, "error", e.Error())
							return
						}

						e = backend.destroy(id)
						if nil == e {
							indexHandlerWithMessage(w, r, "success", "The job was deleted")
						} else {
							indexHandlerWithMessage(w, r, "error", e.Error())
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
							indexHandlerWithMessage(w, r, "error", e.Error())
							return
						}

						e = backend.destroy(id)
						if nil == e {
							indexHandlerWithMessage(w, r, "success", "The job was deleted")
						} else {
							indexHandlerWithMessage(w, r, "error", e.Error())
						}
						return
					}
				}
			}

			w.WriteHeader(http.StatusNotFound)
		})
	log.Println("[delayed_job] serving at '" + *listenAddress + "'")
	http.ListenAndServe(*listenAddress, nil)
}
