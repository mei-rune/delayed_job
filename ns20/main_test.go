package ns20

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"golang.org/x/text/transform"
)

func TestSimple(t *testing.T) {

	var phone = "13811601607"

	txt := "30003100310031003300380031003100360030003100360030003700604f7d59"

	bs, err := hex.DecodeString(txt)
	if err != nil {
		t.Error(err)
		return
	}

	newBs, _, err := transform.Bytes(charset.NewDecoder(), bs)
	if err != nil {
		t.Error(err)
		return
	}

	txtMessage := string(newBs)

	if txtMessage != "011"+phone+"你好" {
		t.Error("want:", "011"+phone+"你好")
		t.Error("got :", txtMessage)
		//	return
	}

	outBs, _, err := transform.Bytes(charset.NewEncoder(), []byte("你好"))
	if err != nil {
		t.Error(err)
		return
	}

	outBs, err = EncodeOutgoing(phone, outBs)
	if err != nil {
		t.Error(err)
		return
	}

	if !bytes.Equal(bs, outBs) {
		t.Error("want:", txt)
		t.Error("got :", hex.EncodeToString(outBs))
		return
	}

	responseTxt := "31003a0053004d0053002000730065006e00640020004f004b0021000d000a00"

	responseBs, err := hex.DecodeString(responseTxt)
	if err != nil {
		t.Error(err)
		return
	}

	newResponseBs, _, err := transform.Bytes(charset.NewDecoder(), responseBs)
	if err != nil {
		t.Error(err)
		return
	}

	responseMessage := string(newResponseBs)

	if responseMessage != "1:SMS send OK!\r\n" {
		t.Error("want:", "1:SMS send OK!\r\n", "|")
		t.Error("got :", responseMessage, "|")
		return
	}
}

func TestSend(t *testing.T) {
	t.SkipNow()

	err := Send("192.168.1.24:1234", "", "license到期提醒功能，通过短信或其他通知方式在license到期前一周进行提醒.原因:如在假期,周末,工作忙时客户不会特意记着事情,一担授权过期影响客户使用", 1*time.Minute)
	if err != nil {
		t.Error(err)
		return
	}
}
