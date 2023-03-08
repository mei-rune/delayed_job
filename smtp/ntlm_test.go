package smtp

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/git-lfs/go-ntlm/ntlm"
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
	auth := NTLMAuth("", "aaaaaa/admin1", "1234556", NTLMVersion2)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	to := []string{"testpang@mychery.com"}
	msg := []byte("This is the email body.")
	err := SendMail("192.168.1.144:25", auth, "admin1@aaaaaa", to, msg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNTLMv1(t *testing.T) {
	// 334 NTLM supported
	// TlRMTVNTUAABAAAAB4IIogAAAAAAAAAAAAAAAAAAAAAGAbEdAAAADw==
	// 334 TlRMTVNTUAACAAAAEAAQADgAAAAFgomieDpqPCf0jgkAAAAAAAAAAJoAmgBIAAAABgGxHQAAAA9TAE8ATABBAFIATwBOAEUAAgAQAFMATwBMAEEAUgBPAE4ARQABAA4AUQBEAEMAQQBTADAAMgAEABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0AAwAoAFEARABDAEEAUwAwADIALgBzAG8AbABhAHIAbwBuAGUALgBjAG8AbQAFABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0ABwAIADfOD5UbXdMBAAAAAA==
	// TlRMTVNTUAADAAAAGAAYAFgAAAAYABgAcAAAABAAEACIAAAAGgAaAJgAAAAAAAAAsgAAAAAAAACyAAAABYKJogAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADsWq0AKXlBuAAAAAAAAAAAAAAAAAAAAAKgX8JCO8iNbEWS4hs53c3ikmFg3Rw47U3MAbwBsAGEAcgBvAG4AZQBhAGQAbQBpAG4AaQBzAHQAcgBhAHQAbwByAA==
	// 535 5.7.3 Authentication unsuccessful
	// *
	// 500 5.3.3 Unrecognized command
	// QUIT
	// 221 2.0.0 Service closing transmission channel

	// AUTH NTLM
	// 334 NTLM supported
	// TlRMTVNTUAABAAAAB4IIogAAAAAAAAAAAAAAAAAAAAAGAbEdAAAADw==
	// 334 TlRMTVNTUAACAAAAEAAQADgAAAAFgomi3VJPak1VIVYAAAAAAAAAAJoAmgBIAAAABgGxHQAAAA9TAE8ATABBAFIATwBOAEUAAgAQAFMATwBMAEEAUgBPAE4ARQABAA4AUQBEAEMAQQBTADAAMgAEABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0AAwAoAFEARABDAEEAUwAwADIALgBzAG8AbABhAHIAbwBuAGUALgBjAG8AbQAFABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0ABwAIACJe/GUdXdMBAAAAAA==
	// TlRMTVNTUAADAAAAGAAYAFgAAAAYABgAcAAAABAAEACIAAAAGgAaAJgAAAAAAAAAsgAAAAAAAACyAAAABYKJogAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAUX4LIsMZnzAAAAAAAAAAAAAAAAAAAAANFLhkGkGOl3/iHXprxgz/cqMhBq2zQB5XMAbwBsAGEAcgBvAG4AZQBhAGQAbQBpAG4AaQBzAHQAcgBhAHQAbwByAA==

	encoding := base64.StdEncoding

	// bs, err := encoding.DecodeString("TlRMTVNTUAABAAAAB4IIogAAAAAAAAAAAAAAAAAAAAAGAbEdAAAADw")
	// if err != nil {
	// 	t.Error(err)
	// 	return
	// }

	// auth, err := ntlm.p(bs)
	// if err != nil {
	// 	t.Error(err)
	// 	return
	// }
	// fmt.Printf("%#v\r\n", auth)

	bs, err := encoding.DecodeString("TlRMTVNTUAADAAAAGAAYAFgAAAAYABgAcAAAABAAEACIAAAAGgAaAJgAAAAAAAAAsgAAAAAAAACyAAAABYKJogAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAUX4LIsMZnzAAAAAAAAAAAAAAAAAAAAANFLhkGkGOl3/iHXprxgz/cqMhBq2zQB5XMAbwBsAGEAcgBvAG4AZQBhAGQAbQBpAG4AaQBzAHQAcgBhAHQAbwByAA==")
	if err != nil {
		t.Error(err)
		return
	}

	auth, err := ntlm.ParseAuthenticateMessage(bs, int(ntlm.Version1))
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Printf("%#v\r\n", auth)
	fmt.Println(string(auth.DomainName.Payload))
	fmt.Println(string(auth.UserName.Payload))
	fmt.Println(string(auth.Workstation.Payload))

	client, _ := ntlm.CreateClientSession(ntlm.Version1, ntlm.ConnectionlessMode)

	bs, err = encoding.DecodeString("TlRMTVNTUAACAAAAEAAQADgAAAAFgomi3VJPak1VIVYAAAAAAAAAAJoAmgBIAAAABgGxHQAAAA9TAE8ATABBAFIATwBOAEUAAgAQAFMATwBMAEEAUgBPAE4ARQABAA4AUQBEAEMAQQBTADAAMgAEABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0AAwAoAFEARABDAEEAUwAwADIALgBzAG8AbABhAHIAbwBuAGUALgBjAG8AbQAFABgAcwBvAGwAYQByAG8AbgBlAC4AYwBvAG0ABwAIACJe/GUdXdMBAAAAAA==")
	if err != nil {
		t.Error(err)
		return
	}
	challenge, err := ntlm.ParseChallengeMessage(bs)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf("%#v\r\n", challenge)

	// "solarone"

	client.SetUserInfo("administrator", "s0lar1!011", "solarone")
	err = client.ProcessChallengeMessage(challenge)
	if err != nil {
		t.Error(err)
		return
	}

	auth, err = client.GenerateAuthenticateMessage()
	if err != nil {
		t.Error(err)
		return
	}

	t.Log(encoding.EncodeToString(auth.Bytes()))
	t.Log("TlRMTVNTUAADAAAAGAAYAFgAAAAYABgAcAAAABAAEACIAAAAGgAaAJgAAAAAAAAAsgAAAAAAAACyAAAABYKJogAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAUX4LIsMZnzAAAAAAAAAAAAAAAAAAAAANFLhkGkGOl3/iHXprxgz/cqMhBq2zQB5XMAbwBsAGEAcgBvAG4AZQBhAGQAbQBpAG4AaQBzAHQAcgBhAHQAbwByAA==")

}
