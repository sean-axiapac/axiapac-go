package main

import (
	"os"

	"axiapac.com/axiapac/infrastructure/communication"
)

func main() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	slack := communication.NewSlack(token, communication.SlackOption{
		InfoChannelID:  "UFEP0K1U2",
		ErrorChannelID: "#devops",
	})
	if err := slack.Info("Some text"); err != nil {
		panic(err)
	}
	if err := slack.Error("test"); err != nil {
		panic(err)
	}

}
