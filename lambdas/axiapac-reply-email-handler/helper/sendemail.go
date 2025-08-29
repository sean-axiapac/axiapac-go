package helper

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

func SendEmail(ctx context.Context, info *EmailInfo) error {
	emailRaw, err := BuildEmailBuffer(info)
	if err != nil {
		return err
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	client := ses.NewFromConfig(cfg)

	// Send via SES
	res, err := client.SendRawEmail(
		ctx,
		&ses.SendRawEmailInput{
			RawMessage: &types.RawMessage{
				Data: emailRaw.Bytes(),
			},
		},
	)

	fmt.Printf("%s", *res.MessageId)
	return err
}

func BuildEmailBuffer(info *EmailInfo) (*bytes.Buffer, error) {
	var emailRaw bytes.Buffer
	writer := multipart.NewWriter(&emailRaw)
	boundary := writer.Boundary()

	// Set headers manually
	headers := fmt.Sprintf("From: %s\r\n", info.From)
	if len(info.To) > 0 {
		headers += fmt.Sprintf("To: %s\r\n", strings.Join(info.To, ", "))
	}
	if len(info.Cc) > 0 {
		headers += fmt.Sprintf("Cc: %s\r\n", strings.Join(info.Cc, ", "))
	}
	if len(info.Bcc) > 0 {
		headers += fmt.Sprintf("Bcc: %s\r\n", strings.Join(info.Bcc, ", "))
	}
	headers += fmt.Sprintf("Subject: %s\r\n", info.Subject)
	headers += "MIME-Version: 1.0\r\n"
	headers += fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	headers += "\r\n"
	emailRaw.WriteString(headers)

	// Create alternative part (text/plain + text/html)
	altBuf := &bytes.Buffer{}
	altWriter := multipart.NewWriter(altBuf)
	altBoundary := altWriter.Boundary()

	altHeaders := textproto.MIMEHeader{}
	altHeaders.Set("Content-Type", "multipart/alternative; boundary="+altBoundary)
	altPart, _ := writer.CreatePart(altHeaders)

	// Text part
	if info.Text != "" {
		part, _ := altWriter.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {"text/plain; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		qp := quotedprintable.NewWriter(part)
		qp.Write([]byte(info.Text))
		qp.Close()
	}

	// HTML part
	if info.HTML != "" {
		part, _ := altWriter.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {"text/html; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		qp := quotedprintable.NewWriter(part)
		qp.Write([]byte(info.HTML))
		qp.Close()
	}

	altWriter.Close()
	altPart.Write(altBuf.Bytes())

	// Attachments
	for _, att := range info.Attachments {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", fmt.Sprintf("%s; name=\"%s\"", att.ContentType, att.Filename))
		h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", att.Filename))
		h.Set("Content-Transfer-Encoding", "base64")

		part, _ := writer.CreatePart(h)
		b := make([]byte, base64.StdEncoding.EncodedLen(len(att.Content)))
		base64.StdEncoding.Encode(b, att.Content)

		// wrap lines at 76 chars
		for i := 0; i < len(b); i += 76 {
			end := i + 76
			if end > len(b) {
				end = len(b)
			}
			part.Write(b[i:end])
			part.Write([]byte("\r\n"))
		}
	}

	writer.Close()

	return &emailRaw, nil
}
