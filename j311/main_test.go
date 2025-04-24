package j311

import (
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	err := SendMessage("192.168.1.19:8234", 30*time.Second, SMS, "GB2312", "133xxxxxx", "客户呀啊")
	if err != nil {
		t.Error(err)
	}
}

func TestSendTts(t *testing.T) {
	err := SendMessage("192.168.1.19:8234", 30*time.Second, TTS, "GB2312", "133xxxxxx", "test")
	if err != nil {
		t.Error(err)
	}
}
