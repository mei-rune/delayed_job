package delayed_job

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type DBPlugin interface {
	Name() string
	TransformUrl(options map[string]string) (string, error)
	Exec(urlStr, scripts string) error
}

var db_plugins []DBPlugin

func RegisterDBPlugin(plugin DBPlugin) {
	db_plugins = append(db_plugins, plugin)
}

type dbHandler struct {
	drv    string
	urlStr string

	script string
}

func parseUrl(urlStr string) (map[string]string, error) {
	options := map[string]string{}
	if 0 == len(urlStr) {
		return options, nil
	}

	urlStr = strings.TrimSpace(urlStr)

	var ss []string
	var sb bytes.Buffer
	isEscape := false
	for _, c := range urlStr {
		switch c {
		case '\\':
			if isEscape {
				isEscape = false
				sb.WriteRune('\\')
			} else {
				isEscape = true
			}
		case ';':
			if isEscape {
				isEscape = false
				sb.WriteRune(';')
			} else {
				ss = append(ss, sb.String())
				sb.Reset()
			}
		default:
			if isEscape {
				isEscape = false
				sb.WriteRune('\\')
			}
			sb.WriteRune(c)
		}
	}
	if sb.Len() > 0 {
		ss = append(ss, sb.String())
	}

	for _, p := range ss {
		if "" == p {
			continue
		}

		kv := strings.SplitN(p, "=", 2)
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

func transformUrl(drv, urlStr string) (string, error) {
	if !strings.HasPrefix(urlStr, "gdbc:") {
		return urlStr, nil
	}
	urlStr = strings.TrimPrefix(urlStr, "gdbc:")
	options, e := parseUrl(urlStr)
	if nil != e {
		return "", e
	}

	switch drv {
	case "sqlserver", "mssql":
		host, port, dbname, user, password, args, e := fetchArguments(options)
		if nil != e {
			return "", e
		}

		query := url.Values{}
		query.Add("database", dbname)
		query.Add("connection timeout", "30")
		query.Add("app name", "tpt_delay_jobs")
		for key, value := range args {
			query.Set(key, value)
		}
		u := &url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(user, password),
			Host:   net.JoinHostPort(host, port),
			// Path:  instance, // if connecting to an instance instead of a port
			RawQuery: query.Encode(),
		}
		return u.String(), nil
	case "oracle":
		host, port, dbname, user, password, args, e := fetchArguments(options)
		if nil != e {
			return "", e
		}
		
		query := url.Values{}
		// query.Add("database", db.DbName)
		// query.Add("connection timeout", "30")
		// query.Add("app name", "moo")
		for key, value := range args {
			query.Set(key, value)
		}

		u := &url.URL{
			Scheme:   "oracle",
			User:     url.UserPassword(user, password),
			Host:     net.JoinHostPort(host, port),
			Path:     dbname,
			RawQuery: query.Encode(),
		}
		return u.String(), nil
	case "postgres", "opengauss", "kingbase":
		host, port, dbname, user, password, _, e := fetchArguments(options)
		if nil != e {
			return "", e
		}
		return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
			host, port, dbname, user, password), nil
	case "mysql":
		host, port, dbname, user, password, args, e := fetchArguments(options)
		if nil != e {
			return "", e
		}
		var buffer bytes.Buffer
		fmt.Fprintf(&buffer, "%s:%s@tcp(%s:%s)/%s?autocommit=true&parseTime=true",
			user, password, host, port, dbname)
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

		fmt.Fprintf(&buffer, "tcp:%s:%s%s*%s/%s/%s",
			host, port, buffer.String(),
			dbname,
			user,
			password)
		return buffer.String(), nil
	case "oci8":
		tns_name, ok := options["tns"]
		if !ok || 0 == len(tns_name) {
			return "", errors.New("'tns' is required in the url")
		}
		delete(options, "tns")
		uid, ok := options["user"]
		if !ok || 0 == len(uid) {
			return "", errors.New("'user' is required in the url")
		}
		delete(options, "user")
		password, ok := options["password"]
		if !ok || 0 == len(password) {
			return "", errors.New("'password' is required in the url")
		}
		delete(options, "password")
		var buffer bytes.Buffer
		//system/123456@TPT
		fmt.Fprintf(&buffer, "%s/%s@%s",
			uid, password, tns_name)
		for k, v := range options {
			buffer.WriteString(";")
			buffer.WriteString(k)
			buffer.WriteString("=")
			buffer.WriteString(v)
		}
		return buffer.String(), nil
	default:
		if strings.HasPrefix(drv, "odbc_with_") {
			dsn_name, ok := options["dsn"]
			if !ok || 0 == len(dsn_name) {
				return "", errors.New("'dsn' is required in the url")
			}
			delete(options, "dsn")
			uid, ok := options["user"]
			if !ok || 0 == len(uid) {
				return "", errors.New("'user' is required in the url")
			}
			delete(options, "user")
			password, ok := options["password"]
			if !ok || 0 == len(password) {
				return "", errors.New("'password' is required in the url")
			}
			delete(options, "password")
			var buffer bytes.Buffer
			fmt.Fprintf(&buffer, "DSN=%s;UID=%s;PWD=%s",
				dsn_name, uid, password)
			for k, v := range options {
				buffer.WriteString(";")
				buffer.WriteString(k)
				buffer.WriteString("=")
				buffer.WriteString(v)
			}
			return buffer.String(), nil
		}
		for _, plugin := range db_plugins {
			if plugin.Name() == drv {
				return plugin.TransformUrl(options)
			}
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

	drv := stringWithDefault(params, "drv", "") // GetTestConnDrv())
	if 0 == len(drv) {
		// drv = GetTestConnDrv()
		//if 0 == len(drv) {
		return nil, errors.New("'drv' is required")
		//}
	}

	urlStr := stringWithDefault(params, "url", "") // GetTestConnURL())
	if 0 == len(urlStr) {
		// urlStr = GetTestConnURL()
		//if 0 == len(urlStr) {
		return nil, errors.New("'url' is required")
		//}
	}

	script := stringWithDefault(params, "script", "")
	if 0 == len(script) {
		return nil, errors.New("'script' is required")
	}

	var e error
	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)

		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}

		script, e = genText(script, args)
		if nil != e {
			return nil, errors.New("'script' is invalid, " + e.Error())
		}

		urlStr, e = genText(urlStr, args)
		if nil != e {
			return nil, errors.New("'url' is invalid, " + e.Error())
		}

		urlStr, e = transformUrl(drv, urlStr)
		if nil != e {
			return nil, errors.New("'url' is invalid, " + e.Error())
		}
	} else {
		urlStr, e = transformUrl(drv, urlStr)
		if nil != e {
			return nil, errors.New("'url' is invalid, " + e.Error())
		}
	}

	return &dbHandler{drv: drv, urlStr: urlStr, script: script}, nil
}

func (self *dbHandler) Perform() (err error) {
	dbType := ToDbType(self.drv)
	drv := self.drv
	if strings.HasPrefix(self.drv, "odbc_with_") {
		drv = "odbc"
	}

	for _, plugin := range db_plugins {
		if plugin.Name() == drv {
			return plugin.Exec(self.urlStr, self.script)
		}
	}

	db, e := sql.Open(drv, self.urlStr)
	if nil != e {
		return i18n(dbType, self.drv, e)
	}
	defer db.Close()

	if MYSQL == dbType || ORACLE == dbType {
		tx, e := db.Begin()
		if nil != e {
			return errors.New("open transaction failed, " + i18nString(dbType, self.drv, e))
		}
		isCommited := false
		defer func() {
			if !isCommited {
				e := tx.Rollback()
				if nil == err {
					err = errors.New("rollback transaction failed, " + i18nString(dbType, self.drv, e))
				}
			}
		}()

		scaner := bufio.NewScanner(bytes.NewBufferString(self.script))
		scaner.Split(bufio.ScanLines)
		var line string
		for scaner.Scan() {
			line += strings.TrimSpace(scaner.Text())
			if strings.HasSuffix(line, ";") {
				if ORACLE == dbType {
					line = strings.TrimSuffix(line, ";")
				}
				_, e = db.Exec(line)
				if nil != e {
					return e
				}

				line = ""
			}
		}
		if 0 != len(line) {
			_, e = db.Exec(line)
			if nil != e {
				return i18n(dbType, self.drv, e)
			}
		}

		isCommited = true
		e = tx.Commit()
		if nil != e {
			return errors.New("commit transaction failed, " + i18nString(dbType, self.drv, e))
		}

		return nil
	}

	_, e = db.Exec(self.script)
	if nil != e {
		return i18n(dbType, drv, e)
	}
	return nil
}

func init() {
	Handlers["db"] = newDbHandler
	Handlers["db_command"] = newDbHandler
}
