package delayed_job

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

type dbHandler struct {
	drv string
	url string

	script string
}

func newDbHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == ctx {
		return nil, errors.New("ctx is nil")
	}
	if nil == params {
		return nil, errors.New("params is nil")
	}

	drv := stringWithDefault(params, "drv", *db_drv)
	if 0 == len(drv) {
		drv = *db_drv

		if 0 == len(drv) {
			return nil, errors.New("'drv' is required.")
		}
	}

	url := stringWithDefault(params, "url", *db_url)
	if 0 == len(url) {
		url = *db_url
		if 0 == len(url) {
			return nil, errors.New("'url' is required.")
		}
	}

	script := stringWithDefault(params, "script", "")
	if 0 == len(script) {
		return nil, errors.New("'script' is required.")
	}

	if args, ok := params["arguments"]; ok {
		t, e := template.New("default").Parse(script)
		if nil != e {
			return nil, errors.New("create template failed, " + e.Error())
		}
		var buffer bytes.Buffer
		e = t.Execute(&buffer, args)
		if nil != e {
			return nil, errors.New("execute template failed, " + e.Error())
		}
		script = buffer.String()
	}

	return &dbHandler{drv: drv, url: url, script: script}, nil
}

func (self *dbHandler) Perform() error {
	drv := self.drv
	if strings.HasPrefix(self.drv, "odbc_with_") {
		drv = "odbc"
	}

	db, e := sql.Open(drv, self.url)
	if nil != e {
		return e
	}
	defer db.Close()

	if MYSQL == *db_type {
		scaner := bufio.NewScanner(bytes.NewBufferString(self.script))
		scaner.Split(bufio.ScanLines)
		var line string
		for scaner.Scan() {
			line += strings.TrimSpace(scaner.Text())
			if strings.HasSuffix(line, ";") {
				fmt.Println("execute ", line)
				_, e = db.Exec(line)
				if nil != e {
					return e
				}

				line = ""
			}
		}
		if 0 != len(line) {
			_, e = db.Exec(line)
			return e
		}
		return nil
	}

	_, e = db.Exec(self.script)
	return e
}

func init() {
	Handlers["db"] = newDbHandler
	Handlers["db_command"] = newDbHandler
}
