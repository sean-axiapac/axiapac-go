package helper

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

type EmailInfo struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

func ParseEmail(raw string) (*EmailInfo, error) {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// Subject may be encoded (e.g., =?UTF-8?...), decode if needed
	subject, err := decodeMIMEHeader(msg.Header.Get("Subject"))
	if err != nil {
		subject = msg.Header.Get("Subject") // fallback
	}

	email := &EmailInfo{
		From:    msg.Header.Get("From"),
		To:      msg.Header["To"],
		Cc:      msg.Header["Cc"],
		Bcc:     msg.Header["Bcc"],
		Subject: subject,
	}

	decodeBody := func(r io.Reader, encoding string) ([]byte, error) {
		var reader io.Reader = r
		switch strings.ToLower(encoding) {
		case "base64":
			reader = base64.NewDecoder(base64.StdEncoding, r)
		case "quoted-printable":
			reader = quotedprintable.NewReader(r)
		}
		return io.ReadAll(reader)
	}

	// Check for multipart
	mediatype, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// fallback: plain body
		body, _ := decodeBody(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		email.Text = string(body)
		return email, nil
	}

	if strings.HasPrefix(mediatype, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("multipart read: %w", err)
			}

			slurp, _ := decodeBody(p, p.Header.Get("Content-Transfer-Encoding"))

			ct := p.Header.Get("Content-Type")
			cd := p.Header.Get("Content-Disposition")

			if strings.HasPrefix(ct, "text/plain") {
				email.Text = string(slurp)
			} else if strings.HasPrefix(ct, "text/html") {
				email.HTML = string(slurp)
			} else if strings.HasPrefix(cd, "attachment") {
				_, params, _ := mime.ParseMediaType(cd)
				filename := params["filename"]
				email.Attachments = append(email.Attachments, Attachment{
					Filename:    filename,
					ContentType: ct,
					Content:     slurp,
				})
			}
		}
	} else {
		// not multipart, just single body
		body, _ := decodeBody(msg.Body, msg.Header.Get("Content-Transfer-Encoding"))
		if strings.HasPrefix(mediatype, "text/html") {
			email.HTML = string(body)
		} else {
			email.Text = string(body)
		}
	}

	return email, nil
}

// Decode MIME encoded words in headers (=?UTF-8?...?=)
func decodeMIMEHeader(header string) (string, error) {
	dec := new(mime.WordDecoder)
	return dec.DecodeHeader(header)
}
