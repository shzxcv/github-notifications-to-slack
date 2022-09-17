package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/go-github/github"
	"github.com/kelseyhightower/envconfig"
	"github.com/slack-go/slack"
	"golang.org/x/oauth2"
)

type Env struct {
	GithubToken        string `envconfig:"GITHUB_TOKEN" required:"true"`
	SlackBotOauthToken string `envconfig:"SLACK_BOT_OAUTH_TOKEN" required:"true"`
	SlackChannel       string `envconfig:"SLACK_CHANNEL" required:"false"`
	SlackUserID        string `envconfig:"SLACK_USER_ID" required:"false"`
}

func (e *Env) NewEnv() {
	if err := envconfig.Process("", e); err != nil {
		log.Fatal(err)
	}
}

func main() {
	var e Env
	e.NewEnv()
	err := notifications(&e)
	fmt.Println(err)
}

func notifications(e *Env) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: e.GithubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	notifications, _, err := client.Activity.ListNotifications(ctx, nil)
	if err != nil {
		return err
	}
	for _, n := range notifications {
		subjects := n.GetSubject()
		fmt.Println(subjects.GetLatestCommentURL())
	}
	return nil
}

func send(text string, e *Env) error {
	if e.SlackUserID == "" && e.SlackChannel == "" {
		return errors.New("SlackUserID and SlackChannel is empty")
	}
	var channelID string
	if e.SlackUserID != "" && e.SlackChannel != "" {
		// Direct Message has priority
		channelID = e.SlackUserID
	}
	if e.SlackUserID == "" && e.SlackChannel != "" {
		channelID = e.SlackUserID
	}
	if e.SlackUserID != "" && e.SlackChannel == "" {
		channelID = e.SlackUserID
	}
	client := slack.New(e.SlackBotOauthToken)
	_, _, err := client.PostMessage(channelID, slack.MsgOptionText(text, true))
	if err != nil {
		return err
	}
	return nil
}
