package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

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

type Notification struct {
	Reason string
	URL    string
}

func (e *Env) NewEnv() {
	if err := envconfig.Process("", e); err != nil {
		log.Fatal(err)
	}
}

func main() {
	var e Env
	e.NewEnv()
	n, err := notifications(&e)
	if err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, s := range n {
		wg.Add(1)
		go func(s Notification) {
			block := newBlock(&s)
			err := send(block, &e)
			if err != nil {
				log.Fatal(err)
			}
			wg.Done()
		}(s)
	}
	wg.Wait()
}

func notifications(e *Env) ([]Notification, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: e.GithubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	ns, _, err := client.Activity.ListNotifications(ctx, nil)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	mutex := &sync.Mutex{}
	var result []Notification
	for _, n := range ns {
		wg.Add(1)
		go func(n *github.Notification) error {
			r := n.GetReason()
			s := n.GetSubject()
			url, err := request(s.GetURL(), s.GetType(), e)
			if err != nil {
				return err
			}
			mutex.Lock()
			result = append(result, Notification{Reason: r, URL: url})
			mutex.Unlock()
			wg.Done()
			return nil
		}(n)
	}
	wg.Wait()
	return result, nil
}

func request(url, types string, e *Env) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.GithubToken))
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	var mapBody interface{}
	if err = json.Unmarshal(body, &mapBody); err != nil {
		return "", err
	}
	var resultURL string
	if types == "PullRequest" {
		resultURL = mapBody.(map[string]interface{})["_links"].(map[string]interface{})["html"].(map[string]interface{})["href"].(string)
	} else if types == "Issue" {
		resultURL = mapBody.(map[string]interface{})["html_url"].(string)
	}
	return resultURL, nil
}

func newBlock(n *Notification) *slack.MsgOption {
	text := fmt.Sprintf("*Notification [%s]*\n%s", n.Reason, n.URL)
	block := slack.MsgOptionBlocks(
		&slack.SectionBlock{
			Type: slack.MBTSection,
			Text: &slack.TextBlockObject{Type: "mrkdwn", Text: text},
		},
	)
	return &block
}

func send(block *slack.MsgOption, e *Env) error {
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
	params := slack.PostMessageParameters{
		UnfurlMedia: true,
		UnfurlLinks: true,
	}
	_, _, err := client.PostMessage(channelID, *block, slack.MsgOptionPostMessageParameters(params))
	if err != nil {
		return err
	}
	return nil
}
