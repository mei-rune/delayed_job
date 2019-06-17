package ns20

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var charset = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

// 30 00 31 00 31 00 31 00 33 00 33 00 31 00 31 00 36 00 30 00 31 00 36 00 30 00 38 00 60 4f 7d 59

func EncodeOutgoing(phone, content string) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("0")
	fmt.Fprintf(&sb, "%02d", len(phone))
	sb.WriteString(phone)
	sb.WriteString(content)

	bs, _, err := transform.Bytes(charset.NewEncoder(), []byte(sb.String()))
	return bs, err
}

func DecodeIncoming(in []byte) (string, error) {
	bs, _, err := transform.Bytes(charset.NewDecoder(), in)
	if err != nil {
		return "", err
	}

	txt := string(bs)
	if !strings.HasPrefix(txt, "1:") {
		return "", errors.New(txt)
	}
	return txt, nil
}

func Send(address, phone, content string, timeout time.Duration) error {
	for {
		if len(content) <= 70 {
			return send(address, phone, content, timeout)
		}

		if err := send(address, phone, content[:70], timeout); err != nil {
			return err
		}
		content = content[70:]
	}
}

func send(address, phone, content string, timeout time.Duration) error {
	bs, err := EncodeOutgoing(phone, content)
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := conn.Write(bs)
	if err != nil {
		return err
	}

	if len(bs) != n {
		return errors.New("short write to " + address)
	}

	conn.SetReadDeadline(time.Now().Add(timeout))

	var in = make([]byte, 1024)
	n, err = conn.Read(in)
	if err != nil {
		return err
	}

	if n == 0 {
		return errors.New("short read to " + address)
	}

	_, err = DecodeIncoming(in[:n])
	if err != nil {
		return err
	}
	return nil
}
