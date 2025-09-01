package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"axiapac.com/axiapac/lambdas/axiapac-reply-email-handler/helper"
)

func ProcessReplyEmailNotification(path string) error {
	// get notification text
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// parse notification
	info, err := helper.ParseMessage(string(raw))
	if err != nil {
		return err
	}

	// parse email
	rawEmail, err := base64.StdEncoding.DecodeString(info.Content)
	if err != nil {
		return err
	}
	email, err := helper.ParseEmail(string(rawEmail))
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", email.HTML)
	fmt.Printf("%s\n", info.Mail.Destination)
	// ForwardTo(email, "sean.tang@axiapac.com.au")
	return nil
}

func CheckReplyEmailFromS3(messageId string) (*helper.EmailInfo, error) {
	ctx := context.Background()
	raw, err := helper.GetRawEmailFromS3(ctx, messageId)
	if err != nil {
		return nil, err
	}
	email, err := helper.ParseEmail(string(raw))
	if err != nil {
		return nil, err
	}
	return email, nil
}

func main() {
	// err := ProcessReplyEmailNotification("./message.txt")
	// messageID := "uq63c20thts486b984ri2piavcdbtnpt76mjk101" // plain text email example
	messageID := "ll6jn9df5j1rt4a3g9vku684cu324m2npm5ho8g1" // multipart/report email example
	email, err := CheckReplyEmailFromS3(messageID)

	if err != nil {
		fmt.Printf("%v", err)
	}

	fmt.Printf("From: %s\n", email.From)
	fmt.Printf("To: %s\n", strings.Join(email.To, ", "))
	fmt.Printf("Text: %s\n", email.Text)
	fmt.Println("OK")

	// fmt.Printf("HTML: %s\n", email.HTML)
	// f, _ := helper.FindCustomerEmailByName("wtlmgt")

	// fmt.Println(f)
	// if err := helper.ForwardTo(email, f, &messageID, true); err != nil {
	// 	panic(err)
	// }
	// if err := helper.ForwardTo(email, "sean.tang@axiapac.com.au", &messageID, false); err != nil {
	// 	panic(err)
	// }
}
