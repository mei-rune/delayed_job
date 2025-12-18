package delayed_job

import (
	"regexp"
	"testing"
)

// TestReplaceIPs 测试IP替换功能
func TestReplaceIPs(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		replaceFunc func(ip string) string
		expected    string
		wantErr     bool
	}{
		{
			name: "正常IP替换为掩码",
			text: "服务器IP是192.168.1.1和10.0.0.1",
			replaceFunc: func(ip string) string {
				return "***.***.***.***"
			},
			expected: "服务器IP是***.***.***.***和***.***.***.***",
			wantErr:  false,
		},
		{
			name: "内网外网IP分类替换",
			text: "内网IP: 192.168.1.1, 公网IP: 8.8.8.8",
			replaceFunc: func(ip string) string {
				if regexp.MustCompile(`^(10|172\.(1[6-9]|2[0-9]|3[0-1])|192\.168)\.`).MatchString(ip) {
					return "[内网IP]"
				}
				return "[公网IP]"
			},
			expected: "内网IP: [内网IP], 公网IP: [公网IP]",
			wantErr:  false,
		},
		{
			name: "IP部分隐藏",
			text: "IP地址: 192.168.1.100",
			replaceFunc: func(ip string) string {
				hiddenRegex := regexp.MustCompile(`\.\d+$`)
				return hiddenRegex.ReplaceAllString(ip, ".xxx")
			},
			expected: "IP地址: 192.168.1.xxx",
			wantErr:  false,
		},
		{
			name: "无IP地址的文本",
			text: "这是一段没有IP地址的文本",
			replaceFunc: func(ip string) string {
				return "REPLACED"
			},
			expected: "这是一段没有IP地址的文本",
			wantErr:  false,
		},
		{
			name: "混合内容",
			text: "IP列表: 192.168.1.1, 无效IP: 999.999.999.999, 另一个IP: 10.0.0.1",
			replaceFunc: func(ip string) string {
				return "MASKED"
			},
			expected: "IP列表: MASKED, 无效IP: 999.999.999.999, 另一个IP: MASKED",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ReplaceIPs(tt.text, tt.replaceFunc)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReplaceIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result != tt.expected {
				t.Errorf("ReplaceIPs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestReplaceIPsEdgeCases 测试边界情况
func TestReplaceIPsEdgeCases(t *testing.T) {
	t.Run("空字符串", func(t *testing.T) {
		result, err := ReplaceIPs("", func(ip string) string {
			return "REPLACED"
		})

		if err != nil {
			t.Errorf("空字符串测试出错: %v", err)
		}

		if result != "" {
			t.Errorf("期望空字符串，得到: %s", result)
		}
	})

	t.Run("只有IP的字符串", func(t *testing.T) {
		result, err := ReplaceIPs("192.168.1.1", func(ip string) string {
			return "IP"
		})

		if err != nil {
			t.Errorf("只有IP测试出错: %v", err)
		}

		if result != "IP" {
			t.Errorf("期望'IP'，得到: %s", result)
		}
	})
}

func TestFormatIP(t *testing.T) {
	tests := []struct {
		name        string
		formatStr   string
		ip          string
		expected    int64
		wantErr     bool
		errContains string
	}{
		// 正常情况测试用例
		{
			name:      "保留所有字节",
			formatStr: "1111",
			ip:        "192.168.1.1",
			expected:  192168001001,
			wantErr:   false,
		},
		{
			name:      "保留前两个字节",
			formatStr: "1100",
			ip:        "192.168.1.1",
			expected:  192168,
			wantErr:   false,
		},
		{
			name:      "保留第1和第3个字节",
			formatStr: "1010",
			ip:        "192.168.1.1",
			expected:  192001,
			wantErr:   false,
		},
		{
			name:      "保留第2和第4个字节",
			formatStr: "0101",
			ip:        "192.168.1.1",
			expected:  168001,
			wantErr:   false,
		},
		{
			name:      "只保留第1个字节",
			formatStr: "1000",
			ip:        "192.168.1.1",
			expected:  192,
			wantErr:   false,
		},
		{
			name:      "只保留最后一个字节",
			formatStr: "0001",
			ip:        "192.168.1.1",
			expected:  1,
			wantErr:   false,
		},
		{
			name:      "边界值-最小IP",
			formatStr: "1111",
			ip:        "0.0.0.0",
			expected:  000000000000,
			wantErr:   false,
		},
		{
			name:      "边界值-最大IP",
			formatStr: "1111",
			ip:        "255.255.255.255",
			expected:  255255255255,
			wantErr:   false,
		},
		{
			name:      "包含0的字节处理",
			formatStr: "1111",
			ip:        "10.0.25.1",
			expected:  10000025001,
			wantErr:   false,
		},
		{
			name:      "所有字节都丢弃",
			formatStr: "0000",
			ip:        "192.168.1.1",
			expected:  0,
			wantErr:   false,
		},

		// 错误情况测试用例
		{
			name:        "格式化字符串长度错误-太短",
			formatStr:   "111",
			ip:          "192.168.1.1",
			expected:    0,
			wantErr:     true,
			errContains: "长度必须为4",
		},
		{
			name:        "格式化字符串长度错误-太长",
			formatStr:   "11111",
			ip:          "192.168.1.1",
			expected:    0,
			wantErr:     true,
			errContains: "长度必须为4",
		},
		{
			name:        "格式化字符串含非法字符",
			formatStr:   "11a1",
			ip:          "192.168.1.1",
			expected:    0,
			wantErr:     true,
			errContains: "只能包含'0'或'1'",
		},
		{
			name:        "无效IP地址",
			formatStr:   "1111",
			ip:          "999.999.999.999",
			expected:    0,
			wantErr:     true,
			errContains: "无效的IP地址",
		},
		{
			name:        "IPv6地址",
			formatStr:   "1111",
			ip:          "2001:db8::1",
			expected:    0,
			wantErr:     true,
			errContains: "不支持IPv6地址",
		},
		{
			name:        "空IP地址",
			formatStr:   "1111",
			ip:          "",
			expected:    0,
			wantErr:     true,
			errContains: "无效的IP地址",
		},
		{
			name:        "格式字符串含空格",
			formatStr:   "1 11",
			ip:          "192.168.1.1",
			expected:    0,
			wantErr:     true,
			errContains: "只能包含'0'或'1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatIP(tt.formatStr, tt.ip)

			if (err != nil) != tt.wantErr {
				t.Errorf("FormatIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("期望错误，但得到nil错误")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("错误消息不包含预期文本: 期望包含 '%s', 实际错误: '%v'", tt.errContains, err)
				}
			} else {
				if result != tt.expected {
					t.Errorf("FormatIP() = %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// 测试边界情况和特殊场景
func TestFormatIPEdgeCases(t *testing.T) {
	t.Run("单字节IP部分", func(t *testing.T) {
		// 测试每个字节单独处理的情况
		testCases := []struct {
			format string
			ip     string
			want   int64
		}{
			{"1000", "100.200.230.240", 100}, // 只取第一个字节
			{"0100", "100.200.230.240", 200}, // 只取第二个字节
			{"0010", "100.200.230.240", 230}, // 只取第三个字节
			{"0001", "100.200.230.240", 240}, // 只取第四个字节
		}

		for _, tc := range testCases {
			result, err := FormatIP(tc.format, tc.ip)
			if err != nil {
				t.Errorf("处理IP %s 时出错: %v", tc.ip, err)
				continue
			}
			if result != tc.want {
				t.Errorf("格式 %s, IP %s: 得到 %d, 期望 %d", tc.format, tc.ip, result, tc.want)
			}
		}
	})

	t.Run("连续零字节处理", func(t *testing.T) {
		result, err := FormatIP("1111", "0.0.0.0")
		if err != nil {
			t.Fatalf("处理全零IP时出错: %v", err)
		}
		if result != 0 {
			t.Errorf("全零IP期望0, 得到: %d", result)
		}
	})
}

// 并发安全测试
func TestFormatIPConcurrent(t *testing.T) {
	formatStr := "1010"
	ips := []string{"192.168.1.1", "10.0.0.1", "172.16.1.1"}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			ip := ips[index%len(ips)]
			_, err := FormatIP(formatStr, ip)
			if err != nil {
				t.Errorf("并发测试失败: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
