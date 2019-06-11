package ns20

import (
	"errors"
	"net"
	"time"
)

func EncodeOutgoing(phone, content string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func DecodeIncoming(bs []byte) error {
	return errors.New("not implemented")
}

func Send(address, phone, content string, timeout time.Duration) error {
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
	var count = 0
	for {
		n, err := conn.Read(in[count:])
		if err != nil {
			return err
		}

		if n == 0 {
			return errors.New("short read to " + address)
		}
		count += n

		for {
			err := DecodeIncoming(in[:count])
			if err != nil {
				return err
			}
		}
	}
}
