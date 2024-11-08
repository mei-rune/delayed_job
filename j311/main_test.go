package j311

import (
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	err := SendMessage("192.168.1.222:8234", 30*time.Second, "GB2312", "13311601608", "客户呀啊")
	if err != nil {
		t.Error(err)
	}
}
