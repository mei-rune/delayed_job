package smtp

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"testing"
)

func TestCh(t *testing.T) {

	txt := "TlRMTVNTUAABAAAAB4IIogAAAAAAAAAAAAAAAAAAAAAGAbEdAAAADw=="
	//TlRMTVNTUAABAAAAB4IIogAAAAAAAAAAAAAAAAAAAAAGAbEdAAAADw==

	maxLen := base64.StdEncoding.DecodedLen(len(txt))
	dst := make([]byte, maxLen)
	resultLen, err := base64.StdEncoding.Decode(dst, []byte(txt))
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(dst[:resultLen])
	fmt.Println(string(dst[:resultLen]))

	flags := binary.LittleEndian.Uint32(dst[12:])
	fmt.Println(binary.LittleEndian.Uint32(dst[12:]), NEGOTIATE_FLAGS)

	fmt.Println("NEGOTIATE_UNICODE                  =", (flags&0x00000001) != 0)
	fmt.Println("NEGOTIATE_OEM                      =", (flags&0x00000002) != 0)
	fmt.Println("NEGOTIATE_TARGET                   =", (flags&0x00000004) != 0)
	fmt.Println("NEGOTIATE_SIGN                     =", (flags&0x00000010) != 0)
	fmt.Println("NEGOTIATE_SEAL                     =", (flags&0x00000020) != 0)
	fmt.Println("NEGOTIATE_DATAGRAM                 =", (flags&0x00000040) != 0)
	fmt.Println("NEGOTIATE_LMKEY                    =", (flags&0x00000080) != 0)
	fmt.Println("NEGOTIATE_NTLM                     =", (flags&0x00000200) != 0)
	fmt.Println("NEGOTIATE_ANONYMOUS                =", (flags&0x00000800) != 0)
	fmt.Println("NEGOTIATE_OEM_DOMAIN_SUPPLIED      =", (flags&0x00001000) != 0)
	fmt.Println("NEGOTIATE_OEM_WORKSTATION_SUPPLIED =", (flags&0x00002000) != 0)
	fmt.Println("NEGOTIATE_ALWAYS_SIGN              =", (flags&0x00008000) != 0)
	fmt.Println("NEGOTIATE_TARGET_TYPE_DOMAIN       =", (flags&0x00010000) != 0)
	fmt.Println("NEGOTIATE_TARGET_TYPE_SERVER       =", (flags&0x00020000) != 0)
	fmt.Println("NEGOTIATE_EXTENDED_SESSIONSECURITY =", (flags&0x00080000) != 0)
	fmt.Println("NEGOTIATE_IDENTIFY                 =", (flags&0x00100000) != 0)
	fmt.Println("REQUEST_NON_NT_SESSION_KEY         =", (flags&0x00400000) != 0)
	fmt.Println("NEGOTIATE_TARGET_INFO              =", (flags&0x00800000) != 0)
	fmt.Println("NEGOTIATE_VERSION                  =", (flags&0x02000000) != 0)
	fmt.Println("NEGOTIATE_128                      =", (flags&0x20000000) != 0)
	fmt.Println("NEGOTIATE_KEY_EXCH                 =", (flags&0x40000000) != 0)
	fmt.Println("NEGOTIATE_56                       =", (flags&0x80000000) != 0)

	// [0-6  ] 78 84 76 77 83 83 80 NTLMSSP
	// [7    ] 0
	// [8-11 ] 1 0 0 0      NEGOTIATE_MESSAGE
	// [12-15] 7 130 8 162  NEGOTIATE_FLAGS
	// [16-21] 0 0 0 0 0    domain_len
	// [22-25] 0 0 0 0 0    workstation_len
	// [25-29] 0 0 0 0 0
	// [28-32] 0 6 1 177
	// [17-21] 29 0 0 0
	// [17-21] 15
}

func TestNTLMSend(t *testing.T) {
	auth := NTLMAuth("", "testpang", "111@chery", "")

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	to := []string{"testpang@mychery.com"}
	msg := []byte("This is the email body.")
	err := SendMail("ex.mychery.com:25", auth, "testpang@mychery.com", to, msg)
	if err != nil {
		log.Fatal(err)
	}
}
