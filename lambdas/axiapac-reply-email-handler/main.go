package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"axiapac.com/axiapac/infrastructure/communication"
	"axiapac.com/axiapac/lambdas/axiapac-reply-email-handler/helper"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type SesNotification struct {
	NotificationType string                    `json:"notificationType"`
	Mail             events.SimpleEmailMessage `json:"mail"`
	Receipt          any                       `json:"receipt"`
	Content          string                    `json:"content"`
}

func ForwardToCustomer(email *helper.EmailInfo, messageID *string) (string, error) {
	// loop throw each email address until the email is forwarded
	for _, address := range email.To {
		customer := helper.FindCustomerNameFromAxiapacEmail(address)
		if customer == "" {
			continue
		}

		customerEmail, err := helper.FindCustomerEmailByName(customer)
		if err != nil {
			return "", fmt.Errorf("unable to find email email address for customer %s: %w", customer, err)
		}

		to := strings.Join(email.To, ", ")
		if err := helper.ForwardTo(email, customerEmail, messageID, true); err != nil {
			return "", fmt.Errorf("unable to forward email to %s: %w", customerEmail, err)
		}
		fmt.Printf("[INFO] email sent to %s forward to %s\n", to, customerEmail)

		return customerEmail, nil
	}
	return "", fmt.Errorf("no customer matched for forwarded email %s", strings.Join(email.To, ", "))
}

// Lambda handler function
func HandleRequest(ctx context.Context, event events.SNSEvent) error {
	fmt.Printf("[EVENT] %+v\n", event)
	hasError := false

	slack := communication.ConnectSlack()
	for _, record := range event.Records {
		var sesEvent SesNotification
		err := json.Unmarshal([]byte(record.SNS.Message), &sesEvent)
		if err != nil {
			fmt.Printf("[ERROR] failed to parse SES notification: %v\n", err)
			hasError = true
			continue
		}

		// Check if it's an email receiving event
		if sesEvent.NotificationType == "Received" {
			fmt.Printf("[INFO] MessageId: %s\n", sesEvent.Mail.MessageID)

			// parse email
			rawEmail, err := base64.StdEncoding.DecodeString(sesEvent.Content)
			if err != nil {
				fmt.Printf("[ERROR] failed to decode base64 email content: %v\n", err)
				hasError = true
				continue
			}
			email, err := helper.ParseEmail(string(rawEmail))
			if err != nil {
				fmt.Printf("[ERROR] failed to parse email: %v\n", err)
				hasError = true
				continue
			}

			to := strings.Join(email.To, ", ")
			fmt.Printf("[INFO] From: %s\n", email.From)
			fmt.Printf("[INFO] To: %s\n", to)
			fmt.Printf("[INFO] Subject: %s\n", email.Subject)

			// receiving email send to no-reply@email.axiapac.net.au
			if strings.Contains(to, helper.NO_REPLY_EMAIL) {
				// forward to development
				if err := helper.ForwardTo(email, helper.DEV_TEAM_EMAIL, &sesEvent.Mail.MessageID, true); err != nil {
					fmt.Printf("[ERROR] error while sending email to development team: %v\n", err)
					hasError = true
				}
			} else {
				customerEmail, err := ForwardToCustomer(email, &record.SNS.MessageID)
				// fall back to devops team
				if err != nil {
					if err := helper.ForwardTo(email, helper.DEVOPS_TEAM_EMAIL, &sesEvent.Mail.MessageID, true); err != nil {
						fmt.Printf("[ERROR] error while sending email to devops team: %v\n", err)
						hasError = true
					}
				}
				fmt.Printf("[INFO] email forwarded to: %s\n", customerEmail)
			}
			continue
		} else {
			fmt.Printf("[INFO] This is not an inbound email, type: %s\n", sesEvent.NotificationType)
			continue
		}
	}

	if hasError {
		slack.Error("Error occurred while processing reply emails")
		return fmt.Errorf("error while process reply emails")
	}
	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
