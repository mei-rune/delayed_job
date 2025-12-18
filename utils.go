package delayed_job

import (
	"errors"
	"fmt"
	"net"
	"regexp"
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

// ReplaceIPs 从文本中解析出IP地址并按自定义规则替换
func ReplaceIPs(text string, replaceFunc func(ip string) string) (string, error) {
	// 匹配IPv4地址的正则表达式[1,6](@ref)
	ipRegex := `(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`

	// 编译正则表达式[1,11](@ref)
	reg, err := regexp.Compile(ipRegex)
	if err != nil {
		return "", fmt.Errorf("正则表达式编译失败: %v", err)
	}

	// 使用ReplaceAllStringFunc进行替换[9,11](@ref)
	result := reg.ReplaceAllStringFunc(text, replaceFunc)

	return result, nil
}

// FormatIP 根据格式化字符串转换IP地址
// formatStr: 格式化字符串，如"1100"表示保留前两个字节，丢弃后两个字节
// ip: 要处理的IP地址字符串
// 返回值: 转换后的字符串和错误信息
func FormatIP(formatStr, ip string) (int64, error) {
	// 验证格式化字符串
	if len(formatStr) != 4 {
		return 0, fmt.Errorf("格式化字符串长度必须为4，当前长度为%d", len(formatStr))
	}

	for _, char := range formatStr {
		if char != '0' && char != '1' {
			return 0, fmt.Errorf("格式化字符串只能包含'0'或'1'，包含非法字符: %c", char)
		}
	}

	// 解析IP地址
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return 0, fmt.Errorf("无效的IP地址: %s", ip)
	}

	// 转换为IPv4格式
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return 0, fmt.Errorf("不支持IPv6地址: %s", ip)
	}

	// 处理IP地址的每个字节
	var result int64

	for i, action := range formatStr {
		if i >= len(ipv4) {
			break // 防止索引越界
		}

		if action == '1' {
			result = result*1000 + int64(ipv4[i])
		}
	}

	return result, nil
}


// FormatIP 根据格式化字符串转换IP地址
// formatStr: 格式化字符串，如"1100"表示保留前两个字节，丢弃后两个字节
// ip: 要处理的IP地址字符串
// 返回值: 转换后的字符串和错误信息
func FormatIP2(formatStr, sep string, delta []int64,  ip string) (string, error) {
	// 验证格式化字符串
	if len(formatStr) != 4 {
		return "", fmt.Errorf("格式化字符串长度必须为4，当前长度为%d", len(formatStr))
	}

	for _, char := range formatStr {
		if char != '0' && char != '1' {
			return "", fmt.Errorf("格式化字符串只能包含'0'或'1'，包含非法字符: %c", char)
		}
	}

	// 解析IP地址
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("无效的IP地址: %s", ip)
	}

	// 转换为IPv4格式
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return "", fmt.Errorf("不支持IPv6地址: %s", ip)
	}

	// 处理IP地址的每个字节
	var results []string

	for i, action := range formatStr {
		if i >= len(ipv4) {
			break // 防止索引越界
		}

		if action == '1' {
			results = append(results, strconv.FormatInt( int64(ipv4[i]) + delta[i],  10))
		}
	}

	return strings.Join(results, sep), nil
}
