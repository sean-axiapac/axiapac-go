package helper

import (
	"context"
	"fmt"
	"html"
	"strings"
)

func createOriginalInfo(email *EmailInfo, messageID *string, isHtml bool) string {
	var sb strings.Builder
	if isHtml {
		sb.WriteString("---- Original Email Info ----<br>")
		sb.WriteString(fmt.Sprintf("From: %s<br>", html.EscapeString(email.From)))
		sb.WriteString(fmt.Sprintf("To: %s<br>", html.EscapeString(strings.Join(email.To, ", "))))
		sb.WriteString(fmt.Sprintf("Cc: %s<br>", html.EscapeString(strings.Join(email.Cc, ", "))))
		sb.WriteString(fmt.Sprintf("Bcc: %s<br>", html.EscapeString(strings.Join(email.Bcc, ", "))))
		if messageID != nil {
			sb.WriteString(fmt.Sprintf("MessageID: %s<br>", html.EscapeString(*messageID)))
		}
		sb.WriteString("-----------------------------<br><br>")
	} else {
		sb.WriteString("---- Original Email Info ----\n")
		sb.WriteString(fmt.Sprintf("From: %s\n", email.From))
		sb.WriteString(fmt.Sprintf("To: %s\n", strings.Join(email.To, ", ")))
		sb.WriteString(fmt.Sprintf("Cc: %s\n", strings.Join(email.Cc, ", ")))
		sb.WriteString(fmt.Sprintf("Bcc: %s\n", strings.Join(email.Bcc, ", ")))
		if messageID != nil {
			sb.WriteString(fmt.Sprintf("MessageID: %s\n", *messageID))
		}
		sb.WriteString("-----------------------------\n\n")
	}

	return sb.String()
}

func ForwardTo(email *EmailInfo, address string, messageID *string) error {
	if email.HTML != "" {
		email.HTML = createOriginalInfo(email, messageID, true) + email.HTML
	}
	email.Text = createOriginalInfo(email, messageID, false) + email.Text
	email.From = NO_REPLY_EMAIL
	email.To = []string{address}
	email.Cc = []string{}
	email.Bcc = []string{}

	return SendEmail(context.Background(), email)
}
