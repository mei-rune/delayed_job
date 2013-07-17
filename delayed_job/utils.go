package delayed_job

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case int32:
		return int(value)
	case string:
		i, e := strconv.ParseInt(value, 10, 0)
		if nil != e {
			return defaultValue
		}
		return int(i)
	default:
		s := fmt.Sprint(value)
		i, e := strconv.ParseInt(s, 10, 0)
		if nil != e {
			return defaultValue
		}
		return int(i)
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
func stringWithDefault(args map[string]interface{}, key string, defaultValue string) string {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	if s, ok := v.(string); ok && 0 != len(s) {
		return s
	}
	return fmt.Sprint(v)
}

func stringsWithDefault(args map[string]interface{}, key string, defaultValue []string) []string {
	v, ok := args[key]
	if !ok {
		return defaultValue
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	if s, ok := v.(string); ok && 0 != len(s) {
		return strings.Split(s, ",")
	}
	return defaultValue
}
