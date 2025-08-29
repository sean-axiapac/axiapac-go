package communication

import (
	"fmt"
	"os"

	"github.com/slack-go/slack"
)

type Slack struct {
	client  *slack.Client
	options SlackOption
}

type SlackOption struct {
	InfoChannelID  string
	ErrorChannelID string
}

func ConnectSlack() *Slack {
	token := os.Getenv("SLACK_BOT_TOKEN")
	infoCh := os.Getenv("SLACK_INFO_CHANNEL")
	errorCh := os.Getenv("SLACK_ERROR_CHANNEL")

	return NewSlack(token, SlackOption{InfoChannelID: infoCh, ErrorChannelID: errorCh})
}

func NewSlack(token string, options SlackOption) *Slack {
	client := slack.New(token)
	return &Slack{client: client, options: options}
}

func (s *Slack) postMessage(channelID, message string) error {
	_, _, err := s.client.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
		// slack.MsgOptionAttachments(attachment),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		return fmt.Errorf("failed to post message to Slack: %w", err)
	}
	return nil
}

func (this *Slack) Info(message string) error {
	return this.postMessage(this.options.InfoChannelID, message)
}

func (this *Slack) Error(message string) error {
	return this.postMessage(this.options.ErrorChannelID, message)
}
