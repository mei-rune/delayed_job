package muboat

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	// "github.com/goburrow/serial"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/gsm"

	// modelserial "github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"
	"github.com/warthog618/sms"
)

func Connect(address string) (io.ReadWriteCloser, error) {
	// timeout := 30 * time.Second
	// var conn io.ReadWriteCloser
	if _, _, err := net.SplitHostPort(address); err != nil {
		return nil, err
	}
	return net.Dial("tcp", address)
}

func SendMessage(mio io.ReadWriter, hex, verbose, pdumode bool, timeout time.Duration, number string, msg string) error {
	if hex {
		mio = trace.New(mio, trace.WithReadFormat("r: %v"))
	} else if verbose {
		mio = trace.New(mio)
	}

	gopts := []gsm.Option{}
	if !pdumode {
		gopts = append(gopts, gsm.WithTextMode)
	}
	g := gsm.New(at.New(mio, at.WithTimeout(timeout)), gopts...)
	if err := g.Init(); err != nil {
		return err
	}
	if pdumode {
		return sendPDU(g, number, msg)
	}
	mr, err := g.SendShortMessage(number, msg)
	if err != nil {
		// !!! check CPIN?? on failure to determine root cause??  If ERROR 302
		return fmt.Errorf("%v %v\n", mr, err)
	}

	// log.Println("[[OK]]")
	return nil
}

func sendPDU(g *gsm.GSM, number string, msg string) error {
	pdus, err := sms.Encode([]byte(msg), sms.To(number), sms.WithAllCharsets)
	if err != nil {
		return err
	}
	for i, p := range pdus {
		tp, err := p.MarshalBinary()
		if err != nil {
			return err
		}
		mr, err := g.SendPDU(tp)
		if err != nil {
			// !!! check CPIN?? on failure to determine root cause??  If ERROR 302
			return err
		}
		log.Printf("PDU %d(%d): %v\n", len(pdus), i+1, mr)
	}
	// log.Println("[[OK]]")
	return nil
}
