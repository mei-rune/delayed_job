package delayed_job

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var IsDevEnv = false
var ErrDevEnv = errors.New("请注意，这是测试环境是不可以发送信息 想要发信息，请在 tpt_settings 中增加一条记录:\r\n" +
	"  INSERT INTO tpt_settings(name, value) values('send_in_tpt_networks', 'true')")

func boolWithDefault(args map[string]interface{}, key string, defaultValue bool) bool {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	if value, ok := v.(bool); ok {
		return value
	}

	s := fmt.Sprint(v)
	switch s {
	case "1", "true":
		return true
	case "0", "false":
		return false
	default:
		return defaultValue
	}
}

func intWithDefault(args map[string]interface{}, key string, defaultValue int) int {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	return asIntWithDefault(v, defaultValue)
}

func asIntWithDefault(v interface{}, defaultValue int) int {
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case int32:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	case string:
		i, e := strconv.ParseInt(value, 10, 0)
		if nil != e {
			f64, e := strconv.ParseFloat(value, 64)
			if nil != e {
				return defaultValue
			}
			return int(f64)
		}
		return int(i)
	default:
		s := fmt.Sprint(value)
		i, e := strconv.ParseInt(s, 10, 0)
		if nil != e {
			f64, e := strconv.ParseFloat(s, 64)
			if nil != e {
				return defaultValue
			}
			return int(f64)
		}
		return int(i)
	}
}

func asInt64WithDefault(v interface{}, defaultValue int64) int64 {
	switch value := v.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case int32:
		return int64(value)
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case string:
		i, e := strconv.ParseInt(value, 10, 0)
		if nil != e {
			f64, e := strconv.ParseFloat(value, 64)
			if nil != e {
				return defaultValue
			}
			return int64(f64)
		}
		return int64(i)
	default:
		s := fmt.Sprint(value)
		i, e := strconv.ParseInt(s, 10, 0)
		if nil != e {
			f64, e := strconv.ParseFloat(s, 64)
			if nil != e {
				return defaultValue
			}
			return int64(f64)
		}
		return i
	}
}

func durationWithDefault(args map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	switch value := v.(type) {
	case time.Duration:
		return value
	case string:
		i, e := time.ParseDuration(value)
		if nil != e {
			return defaultValue
		}
		return i
	default:
		s := fmt.Sprint(value)
		i, e := time.ParseDuration(s)
		if nil != e {
			return defaultValue
		}
		return i
	}
}

func timeWithDefault(args map[string]interface{}, key string, defaultValue time.Time) time.Time {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	return asTimeWithDefault(v, defaultValue)
}

func asTimeWithDefault(v interface{}, defaultValue time.Time) time.Time {
	switch value := v.(type) {
	case time.Time:
		return value
	case string:
		for _, layout := range []string{time.ANSIC,
			time.UnixDate,
			time.RubyDate,
			time.RFC822,
			time.RFC822Z,
			time.RFC850,
			time.RFC1123,
			time.RFC1123Z,
			time.RFC3339,
			time.RFC3339Nano} {
			t, e := time.Parse(value, layout)
			if nil == e {
				return t
			}
		}
		return defaultValue
	default:
		return defaultValue
	}
}

func stringWithDefault(args map[string]interface{}, key string, defaultValue string) string {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	if nil == v {
		return defaultValue
	}
	if s, ok := v.(string); ok && 0 != len(s) {
		return s
	}
	return fmt.Sprint(v)
}

func stringOrArrayWithDefault(args map[string]interface{}, keys []string, defaultValue string) string {
	var v interface{}
	var ok bool
	for _, key := range keys {
		v, ok = args[key]
		if ok {
			break
		}
	}
	if !ok {
		return defaultValue
	}

	if nil == v {
		return defaultValue
	}
	if s, ok := v.(string); ok && 0 != len(s) {
		return s
	}

	if ii, ok := v.([]interface{}); ok {
		ss := make([]string, len(ii))
		for i, s := range ii {
			ss[i] = fmt.Sprint(s)
		}
		return strings.Join(ss, ",")
	}
	if ss, ok := v.([]string); ok {
		return strings.Join(ss, ",")
	}
	return fmt.Sprint(v)
}

func stringsWithDefault(args map[string]interface{}, key, sep string, defaultValue []string) []string {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	if ii, ok := v.([]interface{}); ok {
		ss := make([]string, len(ii))
		for i, s := range ii {
			ss[i] = fmt.Sprint(s)
		}
		return ss
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	if s, ok := v.(string); ok && 0 != len(s) {
		if 0 == len(sep) {
			return []string{s}
		}
		return strings.Split(s, sep)
	}
	return defaultValue
}

func SplitLines(s string) []string {
	return strings.Split(s, "\n")
}
