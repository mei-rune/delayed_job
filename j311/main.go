package j311

import (
	"bytes"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func GetCharset(charset string) encoding.Encoding {
	switch strings.ToUpper(charset) {
	case "GB2312", "GB18030":
		return simplifiedchinese.GB18030
	case "HZ-GB2312":
		return simplifiedchinese.HZGB2312
	case "GBK":
		return simplifiedchinese.GBK
	case "BIG5":
		return traditionalchinese.Big5
	case "EUC-JP":
		return japanese.EUCJP
	case "ISO2022JP":
		return japanese.ISO2022JP
	case "SHIFTJIS":
		return japanese.ShiftJIS
	case "EUC-KR":
		return korean.EUCKR
	case "UTF8", "UTF-8":
		return encoding.Nop
	case "UTF16-BOM", "UTF-16-BOM":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "UTF16-BE-BOM", "UTF-16-BE-BOM":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "UTF16-LE-BOM", "UTF-16-LE-BOM":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case "UTF16", "UTF-16":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "UTF16-BE", "UTF-16-BE":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "UTF16-LE", "UTF-16-LE":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	//case "UTF32", "UTF-32":
	//	return simplifiedchinese.GBK
	default:
		return nil
	}
}

func SendMessage(address string, timeout time.Duration, charset, number string, msg string) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	c := make(chan byte, 1000)
	go func() {
		var buf [1]byte
		for {
			n, err := conn.Read(buf[:])
			if err != nil {
				log.Println("[j311]", err)
				close(c)
				break
			}

			if n == 0 {
				continue
			}
			select {
			case c <- buf[0]:
			default:
				return
			}
		}
	}()

	var buf bytes.Buffer
	if strings.HasPrefix(number, "0") {
		number = strings.TrimPrefix(number, "0")
	}

	if !strings.HasPrefix(number, "86") {
		number = "86" + number
	}

	buf.WriteString(number)
	buf.WriteString(":0:")

	if charset == "" {
		charset = "GB2312"
	}
	encoding := GetCharset(charset)
	newContent, _, err := transform.Bytes(encoding.NewEncoder(), []byte(msg))
	if err != nil {
		buf.Write([]byte(msg))
	} else {
		buf.Write(newContent)
	}

	log.Println(buf.String(), buf.Bytes())
	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return err
	}
	buf.Reset()

	timer := time.NewTimer(timeout)
	running := true
	for running {
		select {
		case b, ok := <-c:
			if ok {
				buf.WriteByte(b)
				if bytes.Contains(buf.Bytes(), []byte("SMS_SEND_SUCESS")) {
					running = false
				}
			} else {
				return errors.New("disconnected")
			}
		case <-timer.C:
			return errors.New("timeout")
		}
	}
	return nil
}