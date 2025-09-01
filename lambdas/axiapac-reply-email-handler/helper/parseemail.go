package helper

import (
	"bytes"
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
	subject, err := DecodeMIMEHeader(msg.Header.Get("Subject"))
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

	if mediatype == "multipart/report" {
		r, err := ParseMultipartReport(msg.Body, params["boundary"])
		if err != nil {
			return nil, fmt.Errorf("failed to parse multipart/report: %w", err)
		}
		plain, ok := r.HumanReadable["text/plain"]
		if ok {
			email.Text = plain
		}
		html, ok := r.HumanReadable["text/html"]
		if ok {
			email.HTML = html
		}
	} else if strings.HasPrefix(mediatype, "multipart/") {
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
func DecodeMIMEHeader(header string) (string, error) {
	dec := new(mime.WordDecoder)
	return dec.DecodeHeader(header)
}

type ParsedReport struct {
	HumanReadable   map[string]string // text/plain or text/html parts
	DeliveryStatus  string            // message/delivery-status
	OriginalMessage string            // message/rfc822 (if present)
}

func ParseMultipartReport(r io.Reader, boundary string) (*ParsedReport, error) {
	mr := multipart.NewReader(r, boundary)
	report := &ParsedReport{}

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading part: %w", err)
		}

		ctype := p.Header.Get("Content-Type")
		mediatype, params, err := mime.ParseMediaType(ctype)
		if err != nil {
			return nil, fmt.Errorf("parse content-type: %w", err)
		}

		switch {
		case strings.HasPrefix(mediatype, "multipart/"):
			// Recursively handle nested multiparts (e.g. multipart/alternative)
			nested, err := ParseMultipartReport(p, params["boundary"])
			if err != nil {
				return nil, err
			}
			report.HumanReadable = nested.HumanReadable

		case mediatype == "text/plain" || mediatype == "text/html":
			body, _ := io.ReadAll(p)
			// report.HumanReadable = append(report.HumanReadable, string(body))
			if report.HumanReadable == nil {
				report.HumanReadable = make(map[string]string)
			}
			report.HumanReadable[mediatype] = string(body)

		case mediatype == "message/delivery-status":
			body, _ := io.ReadAll(p)
			report.DeliveryStatus = string(body)

		case mediatype == "message/rfc822":
			body, _ := io.ReadAll(p)
			// parse as an email (optional)
			msg, err := mail.ReadMessage(bytes.NewReader(body))
			if err == nil {
				report.OriginalMessage = msg.Header.Get("Subject")
			} else {
				report.OriginalMessage = string(body)
			}

		default:
			// skip unknown parts
			_, _ = io.Copy(io.Discard, p)
		}
	}

	return report, nil
}
