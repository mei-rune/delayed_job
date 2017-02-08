package delayed_job

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	qp "mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/text/transform"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/runner-mei/delayed_job/smtp"
)

const maxLineLen = 76 // RFC 2045
const crlf = "\r\n"

type MailMessage struct {
	From        mail.Address // if From.Address is empty, Config.DefaultFrom will be used
	To          []*mail.Address
	Cc          []*mail.Address
	Bcc         []*mail.Address
	Subject     string
	ContentText string
	ContentHtml string
	Attachments []Attachment
}

type Attachment struct {
	Name    string
	Content io.Reader
}

func toMailString(addr *mail.Address) string {
	if 0 == len(addr.Name) {
		return addr.Address
	}
	return addr.String()
}

// http://tools.ietf.org/html/rfc822
// http://tools.ietf.org/html/rfc2821
func (self *MailMessage) Bytes() ([]byte, error) {
	from := self.From.String()

	buf := bytes.NewBuffer(make([]byte, 0, 10240))
	write := func(what string, recipients []*mail.Address) {
		if nil == recipients {
			return
		}
		if 0 == len(recipients) {
			return
		}
		for i := range recipients {
			if 0 == i {
				buf.WriteString(what)
			} else {
				buf.WriteString(", ")
			}
			buf.WriteString(toMailString(recipients[i]))
		}
		buf.WriteString(crlf)
	}

	fmt.Fprintf(buf, "From: %s%s", from, crlf)
	write("To: ", self.To)
	write("Cc: ", self.Cc)
	write("Bcc: ", self.Bcc)
	fmt.Fprintf(buf, "Date: %s%s", time.Now().UTC().Format(time.RFC1123Z), crlf)
	fmt.Fprintf(buf, "Subject: %s%s", encodeSubject(self.Subject), crlf)

	var parts *Multipart
	var alternative *Multipart

	if len(self.Attachments) > 0 {
		parts = NewMultipart(buf, "multipart/mixed")
		var e error
		alternative, e = parts.AddMultipart("multipart/alternative")
		if nil != e {
			return nil, e
		}
	} else {
		alternative = NewMultipart(buf, "multipart/alternative")
		parts = alternative
	}

	if "" != self.ContentHtml {
		alternative.AddText("text/html; charset=\"utf-8\"", self.ContentHtml)
	}
	if "" != self.ContentText {
		alternative.AddText("text/plain; charset=\"utf-8\"", self.ContentText)
	}
	alternative.Close()

	if len(self.Attachments) > 0 {
		for _, attachment := range self.Attachments {
			e := parts.AddAttachment(AttachmentSytle,
				attachment.Name, "", attachment.Content)
			if nil != e {
				return nil, e
			}
		}
		parts.Close()
	}

	return buf.Bytes(), nil
}

func (self *MailMessage) Send(smtp_server string, auth smtp.Auth) error {
	if nil == self.To || 0 == len(self.To) {
		return errors.New("'to_address' is missing.")
	}

	if 0 == len(smtp_server) {
		smtp_server = *default_smtp_server
		if 0 == len(smtp_server) {
			return errors.New("'smtp_server' is missing or default 'smtp_server' is not set.")
		}
	}

	if !strings.Contains(smtp_server, ":") {
		smtp_server += ":25"
	}

	to := make([]string, len(self.To))
	for i := range self.To {
		to[i] = self.To[i].Address
	}

	from := self.From.Address
	if 0 == len(from) {
		return errors.New("'from_address' is missing or default 'from_address' is not set.")
	}

	body, e := self.Bytes()
	if nil != e {
		return e
	}

	//fmt.Println(string(self.Bytes()))

	e = smtp.SendMail(smtp_server, auth, from, to, body)
	if nil != e {
		err := smtp.SendMail(smtp_server, nil, from, to, body)
		if nil == err {
			return nil
		}
	}
	return e
}

func encodeSubject(txt string) string {
	switch default_mail_subject_encoding {
	case "gb2312_base64":
		return base64StringWithGB2312(txt)
	case "gb2312_qp":
		return qpStringWithGB2312(txt)
	case "gb2312":
		s, _, e := transform.String(simplifiedchinese.GB18030.NewEncoder(), txt)
		if nil != e {
			return qpString(txt)
		}
		return s
	case "utf8_qp":
		return qpString(txt)
	default:
		return qpString(txt)
	}
}

func base64StringWithGB2312(txt string) string {
	buf := bytes.NewBufferString("=?GB2312?B?")
	bs, _, e := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(txt))
	if nil != e {
		return qpString(txt)
	}
	buf.WriteString(base64.StdEncoding.EncodeToString(bs))
	buf.WriteString("?=")
	return buf.String()
}

func qpStringWithGB2312(txt string) string {
	buf := bytes.NewBufferString("=?GB2312?Q?")
	bs, _, e := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte(txt))
	if nil != e {
		return qpString(txt)
	}
	w := qp.NewWriter(buf)
	w.Write(bs)
	w.Close()

	buf.WriteString("?=")
	return buf.String()
}

func qpString(txt string) string {
	buf := bytes.NewBufferString("=?utf-8?q?")
	w := qp.NewWriter(buf)
	io.WriteString(w, txt)
	w.Close()

	buf.WriteString("?=")
	return buf.String()
}

var random_id_gen int32 = 0

func randomString() string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s%d", time.Now().Nanosecond(), atomic.AddInt32(&random_id_gen, 1)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Generate aun unique boundary value
func (self *MailMessage) GetBoundary() string {
	return randomString()
}

// AttachmentType indicates to the mail user agent how an attachment should be
// treated.
type AttachmentType string

const (
	// Attachment indicates that the attachment should be offered as an optional
	// download.
	AttachmentSytle AttachmentType = "attachment"
	// Inline indicates that the attachment should be rendered in the body of
	// the message.
	InlineSytle AttachmentType = "inline"
)

// Multipart repreents a multipart message body. It can other nest multiparts,
// texts, and attachments.
type Multipart struct {
	writer    *multipart.Writer
	mediaType string
	isClosed  bool
	header    textproto.MIMEHeader
}

var ErrPartClosed = errors.New("mail: part has been closed")

// AddMultipart creates a nested part with mediaType and a randomly generated
// boundary. The returned nested part can then be used to add a text or
// an attachment.
//
// Example:
// 	alt, _ := part.AddMultipart("multipart/mixed")
// 	alt.AddText("text/plain", text)
// 	alt.AddAttachment("gopher.png", "", image)
// 	alt.Close()
func (p *Multipart) AddMultipart(mediaType string) (nested *Multipart, err error) {
	if !strings.HasPrefix(mediaType, "multipart") {
		return nil, errors.New("mail: mediaType must start with the word \"multipart\" as in multipart/mixed or multipart/alter")
	}

	if p.isClosed {
		return nil, ErrPartClosed
	}

	boundary := randomString()

	// Mutlipart management
	var mimeType string
	if strings.HasPrefix(mediaType, "multipart") {
		mimeType = mime.FormatMediaType(
			mediaType,
			map[string]string{"boundary": boundary},
		)
	} else {
		mimeType = mediaType
	}

	// Header
	p.header = make(textproto.MIMEHeader)
	p.header["Content-Type"] = []string{mimeType}

	w, err := p.writer.CreatePart(p.header)
	if err != nil {
		return nil, err
	}

	nested = createPart(w, p.header, mediaType, boundary)
	return nested, nil
}

// AddText applies quoted-printable encoding to the content of r before writing
// the encoded result in a new sub-part with media MIME type set to mediaType.
//
// Specifying the charset in the mediaType string is recommended
// ("plain/text; charset=utf-8").
func (p *Multipart) AddTextReader(mediaType string, r io.Reader) error {
	if p.isClosed {
		return ErrPartClosed
	}

	p.header = textproto.MIMEHeader(map[string][]string{
		"Content-Type":              {mediaType},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})

	w, err := p.writer.CreatePart(p.header)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(r)
	encoder := qp.NewWriter(w)
	buffer := make([]byte, maxLineLen)
	for {
		read, err := reader.Read(buffer[:])
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		encoder.Write(buffer[:read])
	}
	encoder.Close()
	fmt.Fprintf(w, crlf)
	fmt.Fprintf(w, crlf)
	return nil
}

func (p *Multipart) AddText(mediaType string, txt string) error {
	if p.isClosed {
		return ErrPartClosed
	}

	p.header = textproto.MIMEHeader(map[string][]string{
		"Content-Type":              {mediaType},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})

	w, err := p.writer.CreatePart(p.header)
	if err != nil {
		return err
	}

	encoder := qp.NewWriter(w)
	_, err = io.WriteString(encoder, txt)
	if err != nil {
		return err
	}

	encoder.Close()

	fmt.Fprintf(w, crlf)
	fmt.Fprintf(w, crlf)
	return nil
}

// AddAttachment encodes the content of r in base64 and writes it as an
// attachment of type attachType in this part.
//
// filename is the file name that will be suggested by the mail user agent to a
// user who would like to download the attachment. It's also the value to which
// the Content-ID header will be set. A name with an extension such as
// "report.docx" or "photo.jpg" is recommended. RFC 5987 is not supported, so
// the charset is restricted to ASCII characters.
//
// mediaType indicates the content type of the attachment. If an empty string is
// passed, mime.TypeByExtension will first be called to deduce a value from the
// extension of filemame before defaulting to "application/octet-stream".
//
// In the following example, the media MIME type will be set to "image/png"
// based on the ".png" extension of the filename "gopher.png":
// 	part.AddAttachment(Inline, "gopher.png", "", image)
func (p *Multipart) AddAttachment(attachType AttachmentType, filename, mediaType string, r io.Reader) (err error) {
	if p.isClosed {
		return ErrPartClosed
	}

	// Default Content-Type value
	if mediaType == "" && filename != "" {
		mediaType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	header := textproto.MIMEHeader(map[string][]string{
		"Content-Type":              {mediaType},
		"Content-ID":                {"<" + filename + ">"},
		"Content-Location":          {filename},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf("%s;\r\n\tfilename=%s;", attachType, qpString(filename))},
	})

	w, err := p.writer.CreatePart(header)
	if err != nil {
		return err
	}

	encoder := base64.NewEncoder(base64.StdEncoding, w)
	data := bufio.NewReader(r)

	buffer := make([]byte, int(math.Ceil(maxLineLen/4)*3))
	for {
		read, err := io.ReadAtLeast(data, buffer[:], len(buffer))
		if err != nil {
			if err == io.EOF {
				break
			} else if err != io.ErrUnexpectedEOF {
				return err
			}
		}

		if _, err := encoder.Write(buffer[:read]); err != nil {
			return err
		}

		if read == len(buffer) {
			fmt.Fprintf(w, crlf)
		}
	}
	encoder.Close()
	fmt.Fprintf(w, crlf)

	return nil
}

// Header map of the part.
func (p *Multipart) Header() textproto.MIMEHeader {
	return p.header
}

// Boundary separating the children of this part.
func (p *Multipart) Boundary() string {
	return p.writer.Boundary()
}

// MediaType returns the media MIME type of this part.
func (p *Multipart) MediaType() string {
	return p.mediaType
}

// Close adds a closing boundary to the part.
//
// Calling AddText, AddAttachment or AddMultipart on a closed part will return
// ErrPartClosed.
func (p *Multipart) Close() error {
	if p.isClosed {
		return ErrPartClosed
	}
	p.isClosed = true
	return p.writer.Close()
}

func createPart(w io.Writer, header textproto.MIMEHeader, mediaType string, boundary string) *Multipart {
	m := &Multipart{
		writer:    multipart.NewWriter(w),
		header:    header,
		mediaType: mediaType,
	}
	m.writer.SetBoundary(boundary)
	return m
}

// NewMultipart modifies msg to become a multipart message and returns the root
// part inside which other parts, texts and attachments can be nested.
//
// Example:
// 	multipart := NewMultipart("multipart/alternative", msg)
// 	multipart.AddPart("text/plain", text)
// 	multipart.AddPart("text/html", html)
// 	multipart.Close()
func NewMultipart(w io.Writer, mediaType string) (root *Multipart) {
	boundary := randomString()
	fmt.Fprintf(w, "Content-Type: %s; boundary=%s%s%s", mediaType, boundary, crlf, crlf)
	return createPart(w, make(textproto.MIMEHeader), mediaType, boundary)
}
