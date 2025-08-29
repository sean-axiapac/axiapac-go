package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"axiapac.com/axiapac/infrastructure/filesystem"
)

// Define the struct matching your JSON
type EmailNotification struct {
	NotificationType string `json:"notificationType"`
	Mail             struct {
		Timestamp        string   `json:"timestamp"`
		Source           string   `json:"source"`
		MessageID        string   `json:"messageId"`
		Destination      []string `json:"destination"`
		HeadersTruncated bool     `json:"headersTruncated"`
		Headers          []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
		CommonHeaders struct {
			From      []string `json:"from"`
			Date      string   `json:"date"`
			To        []string `json:"to"`
			MessageID string   `json:"messageId"`
			Subject   string   `json:"subject"`
		} `json:"commonHeaders"`
	} `json:"mail"`
	Receipt struct {
		Timestamp            string   `json:"timestamp"`
		ProcessingTimeMillis int      `json:"processingTimeMillis"`
		Recipients           []string `json:"recipients"`
		SpamVerdict          struct {
			Status string `json:"status"`
		} `json:"spamVerdict"`
		VirusVerdict struct {
			Status string `json:"status"`
		} `json:"virusVerdict"`
		SpfVerdict struct {
			Status string `json:"status"`
		} `json:"spfVerdict"`
		DkimVerdict struct {
			Status string `json:"status"`
		} `json:"dkimVerdict"`
		DmarcVerdict struct {
			Status string `json:"status"`
		} `json:"dmarcVerdict"`
		Action struct {
			Type     string `json:"type"`
			TopicArn string `json:"topicArn"`
			Encoding string `json:"encoding"`
		} `json:"action"`
	} `json:"receipt"`
	Content string `json:"content"`
}

func ParseMessage(jsonStr string) (*EmailNotification, error) {
	var notification EmailNotification
	err := json.Unmarshal([]byte(jsonStr), &notification)
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func GetRawEmailFromS3(ctx context.Context, messageId string) (string, error) {
	var buf bytes.Buffer
	if err := filesystem.ReadFile("axiapac-devops", fmt.Sprintf("reply-emails/%s", messageId), ctx, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
