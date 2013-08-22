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

func parseUrl(url string) (map[string]string, error) {
	options := map[string]string{}
	if 0 == len(url) {
		return options, nil
	}

	url = strings.TrimSpace(url)
	ps := strings.Split(url, ";")
	for _, p := range ps {
		kv := strings.Split(p, "=")
		if len(kv) < 2 {
			return nil, fmt.Errorf("invalid option: %q", p)
		}
		options[kv[0]] = kv[1]
	}
	return options, nil
}

func fetchArguments(options map[string]string) (host, port, dbname, user, password string, args map[string]string, e error) {
	var ok bool
	host, ok = options["host"]
	if !ok || 0 == len(host) {
		e = errors.New("'host' is required in the url.")
		return
	}
	delete(options, "host")
	port, ok = options["port"]
	if !ok || 0 == len(port) {
		e = errors.New("'port' is required in the url.")
		return
	}
	delete(options, "port")
	user, ok = options["user"]
	if !ok || 0 == len(user) {
		e = errors.New("'user' is required in the url.")
		return
	}
	delete(options, "user")
	password, ok = options["password"]
	if !ok || 0 == len(password) {
		e = errors.New("'password' is required in the url.")
		return
	}
	delete(options, "password")
	dbname, ok = options["dbname"]
	if !ok || 0 == len(dbname) {
		e = errors.New("'dbname' is required in the url.")
		return
	}
	delete(options, "dbname")
	args = options
	return
}

func transformUrl(drv, url string) (string, error) {
	if !strings.HasPrefix(url, "gdbc:") {
		return url, nil
	}
	url = strings.TrimPrefix(url, "gdbc:")
	options, e := parseUrl(url)
	if nil != e {
		return "", e
	}
	switch drv {
	case "postgres":
		host, port, dbname, user, password, _, e := fetchArguments(options)
		if nil != e {
			return "", e
		}
		return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", host, port, dbname, user, password), nil
	case "mysql":
		host, port, dbname, user, password, args, e := fetchArguments(options)
		if nil != e {
			return "", e
		}
		var buffer bytes.Buffer
		fmt.Fprintf(&buffer, "%s:%s@tcp(%s:%s)/%s?autocommit=true&parseTime=true", user, password, host, port, dbname)
		if nil != args && 0 != len(args) {
			for k, v := range args {
				buffer.WriteString("&")
				buffer.WriteString(k)
				buffer.WriteString("=")
				buffer.WriteString(v)
			}
		}
		return buffer.String(), nil
	case "mymysql":
		host, port, dbname, user, password, args, e := fetchArguments(options)
		if nil != e {
			return "", e
		}

		var buffer bytes.Buffer
		if nil != args && 0 != len(args) {
			for k, v := range args {
				buffer.WriteString(",")
				buffer.WriteString(k)
				buffer.WriteString("=")
				buffer.WriteString(v)
			}
		}

		fmt.Fprintf(&buffer, "tcp:%s:%s%s*%s/%s/%s", host, port, buffer.String(), dbname, user, password)

		return buffer.String(), nil
	default:
		if strings.HasPrefix(drv, "odbc_with_") {
			dsn_name, ok := options["dsn"]
			if !ok || 0 == len(dsn_name) {
				return "", errors.New("'dsn' is required in the url.")
			}
			delete(options, "dsn")
			uid, ok := options["user"]
			if !ok || 0 == len(uid) {
				return "", errors.New("'user' is required in the url.")
			}
			delete(options, "user")
			password, ok := options["password"]
			if !ok || 0 == len(password) {
				return "", errors.New("'password' is required in the url.")
			}
			delete(options, "password")
			var buffer bytes.Buffer
			fmt.Fprintf(&buffer, "DSN=%s;UID=%s;PWD=%s", dsn_name, uid, password)
			for k, v := range options {
				buffer.WriteString(";")
				buffer.WriteString(k)
				buffer.WriteString("=")
				buffer.WriteString(v)
			}
			return buffer.String(), nil
		}
		return "", errors.New("unsupported driver - " + drv)
	}
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
	var e error
	url, e = transformUrl(drv, url)
	if nil != e {
		return nil, errors.New("'url' is invalid, " + e.Error())
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
