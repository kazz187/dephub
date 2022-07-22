package main

import (
	"fmt"
	"os"

	"github.com/slack-go/slack"
)

type Slack struct {
	channelID string
	client    *slack.Client
}

func NewSlack(channelID string) *Slack {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		return &Slack{}
	}

	return &Slack{
		channelID: channelID,
		client:    slack.New(token),
	}
}

func (s *Slack) Post(msg string) error {
	if s.client == nil {
		fmt.Println(msg)
		return nil
	}
	_, _, err := s.client.PostMessage(s.channelID, slack.MsgOptionText(msg, false))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}
