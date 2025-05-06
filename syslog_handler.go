package delayed_job

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

var (
	Hostname, _      = os.Hostname()
	default_facility = flag.String("syslog.facility", "user", "the facility of syslog message.")
	default_severity = flag.String("syslog.severity", "info", "the severity of syslog message.")
	default_tag      = flag.String("syslog.tag", "tpt", "the tag of syslog message.")
)

const (
	F_Kernel int = iota
	F_User
	F_Mail
	F_Daemon
	F_Auth
	F_Syslog
	F_Lpr
	F_News
	F_Uucp
	F_Cron
	F_Authpriv
	F_System0
	F_System1
	F_System2
	F_System3
	F_System4
	F_Local0
	F_Local1
	F_Local2
	F_Local3
	F_Local4
	F_Local5
	F_Local6
	F_Local7
)

var facility_2_string = [...]string{
	"kernel",
	"user",
	"mail",
	"daemon",
	"auth",
	"syslog",
	"lpr",
	"news",
	"uucp",
	"cron",
	"authpriv",
	"system0",
	"system1",
	"system2",
	"system3",
	"system4",
	"local0",
	"local1",
	"local2",
	"local3",
	"local4",
	"local5",
	"local6",
	"local7",
}

var string_2_facility = map[string]int{
	"kernel":   F_Kernel,
	"user":     F_User,
	"mail":     F_Mail,
	"daemon":   F_Daemon,
	"auth":     F_Auth,
	"syslog":   F_Syslog,
	"lpr":      F_Lpr,
	"news":     F_News,
	"uucp":     F_Uucp,
	"cron":     F_Cron,
	"authpriv": F_Authpriv,
	"system0":  F_System0,
	"system1":  F_System1,
	"system2":  F_System2,
	"system3":  F_System3,
	"system4":  F_System4,
	"local0":   F_Local0,
	"local1":   F_Local1,
	"local2":   F_Local2,
	"local3":   F_Local3,
	"local4":   F_Local4,
	"local5":   F_Local5,
	"local6":   F_Local6,
	"local7":   F_Local7,
}

const (
	S_Emerg int = iota
	S_Alert
	S_Crit
	S_Err
	S_Warning
	S_Notice
	S_Info
	S_Debug
)

var severity_2_string = [...]string{
	"emerg",
	"alert",
	"crit",
	"err",
	"warning",
	"notice",
	"info",
	"debug",
}

var string_2_severity = map[string]int{
	"emerg":   S_Emerg,
	"alert":   S_Alert,
	"crit":    S_Crit,
	"err":     S_Err,
	"warning": S_Warning,
	"notice":  S_Notice,
	"info":    S_Info,
	"debug":   S_Debug,
}

func message_printf(facility int,
	severity int,
	timestamp time.Time, // optional
	hostname string, // optional
	tag string, // message tag as defined in RFC 3164
	content string) string { // message content as defined in RFC 3164

	return fmt.Sprintf("<%d>%s %s %s %s", facility*8+severity,
		timestamp.Format(time.Stamp), hostname, tag, content)
}

type syslogHandler struct {
	to      []*net.UDPAddr
	message string
}

// <property name="facility">
// <property name="severity">
// <property name="tag">
// <property name="content">

func newSyslogHandler(ctx, params map[string]interface{}) (Handler, error) {
	if nil == params {
		return nil, errors.New("params is nil")
	}

	to := stringsWithDefault(params, "to_address", ",", nil)
	if nil == to || 0 == len(to) {
		return nil, errors.New("'to_address' is required.")
	}

	to_addr := make([]*net.UDPAddr, 0, len(to))
	for _, t := range to {
	reparse:
		addr, e := net.ResolveUDPAddr("udp", t)
		if nil == e && nil != addr {
			to_addr = append(to_addr, addr)
		} else if ip := net.ParseIP(t); nil != ip {
			t += ":514"
			goto reparse
		}
	}
	if 0 == len(to_addr) {
		return nil, errors.New("'to_address' is empty or invalid.")
	}

	facility_s := stringWithDefault(params, "facility", *default_facility)
	if 0 == len(facility_s) {
		return nil, errors.New("'facility' is required")
	}
	facility, ok := string_2_facility[facility_s]
	if !ok {
		return nil, errors.New("'facility' is invalid - '" + facility_s + "'.")
	}
	severity_s := stringWithDefault(params, "severity", *default_severity)
	if 0 == len(severity_s) {
		return nil, errors.New("'severity' is required")
	}
	severity, ok := string_2_severity[severity_s]
	if !ok {
		return nil, errors.New("'severity' is invalid - '" + severity_s + "'.")
	}

	timestamp := timeWithDefault(params, "timestamp", time.Now())

	hostname := stringWithDefault(params, "hostname", Hostname)
	if 0 == len(hostname) {
		return nil, errors.New("'hostname' is required")
	}

	if strings.ContainsAny(hostname, " \t\r\n") {
		return nil, errors.New("'hostname' is invalid - '" + hostname + "'")
	}

	tag := stringWithDefault(params, "tag", *default_tag)
	//if 0 == len(tag) {
	//	return nil, errors.New("'tag' is required.")
	//}

	content := stringWithDefault(params, "content", "")
	if 0 == len(content) {
		return nil, errors.New("'content' is required")
	}

	if args, ok := params["arguments"]; ok {
		args = preprocessArgs(args)
		if props, ok := args.(map[string]interface{}); ok {
			if _, ok := props["self"]; !ok {
				props["self"] = params
				defer delete(props, "self")
			}
		}
		var e error
		content, e = genText(content, args)
		if nil != e {
			return nil, e
		}
	}

	return &syslogHandler{to: to_addr, message: message_printf(facility,
		severity,
		timestamp, // optional
		hostname,  // optional
		tag,       // message tag as defined in RFC 3164
		content)}, nil
}

func (self *syslogHandler) Perform() error {
	if IsDevEnv {
		return ErrDevEnv
	}

	buf := bytes.NewBuffer(make([]byte, 0, 1000))
	hasOk := false
	for _, to := range self.to {
		e := self.send(to)
		if nil == e {
			hasOk = true
		} else {
			buf.WriteString(e.Error())
			buf.WriteString("\r\n")
		}
	}
	if hasOk {
		return nil
	}

	if buf.Len() > 2 {
		buf.Truncate(buf.Len() - 2)
	}
	return errors.New(buf.String())
}

func (self *syslogHandler) send(to *net.UDPAddr) error {
	c, e := net.DialUDP("udp", nil, to)
	if nil != e {
		return e
	}
	defer c.Close()

	fmt.Println(c.RemoteAddr(), self.message)
	_, e = c.Write([]byte(self.message))
	return e
}

func init() {
	Handlers["syslog"] = newSyslogHandler
	Handlers["syslog_command"] = newSyslogHandler
}
