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
	NotificationGithubToken string   `envconfig:"NOTIFICATION_GITHUB_TOKEN" required:"true"`
	SlackBotOauthToken      string   `envconfig:"SLACK_BOT_OAUTH_TOKEN" required:"true"`
	SlackChannel            string   `envconfig:"SLACK_CHANNEL" required:"false"`
	SlackUserID             string   `envconfig:"SLACK_USER_ID" required:"false"`
	IncludeGithubRepos      []string `envconfig:"INCLUDE_GITHUB_REPOS" required:"false"`
	ExcludeGithubRepos      []string `envconfig:"EXCLUDE_GITHUB_REPOS" required:"false"`
}

type Notification struct {
	Reason string
	URL    string
	Title  string
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
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: e.NotificationGithubToken})
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
			t := s.GetTitle()
			repo := n.GetRepository().GetFullName()
			fmt.Println(repo)
			fmt.Println(repoValidator(e.IncludeGithubRepos, e.ExcludeGithubRepos, repo))
			if !repoValidator(e.IncludeGithubRepos, e.ExcludeGithubRepos, repo) {
				wg.Done()
				return nil
			}
			var reqURL string
			if s.GetLatestCommentURL() != "" && (r == "mention" || r == "comment") {
				reqURL = s.GetLatestCommentURL()
			} else {
				reqURL = s.GetURL()
			}
			url, err := request(reqURL, e)
			if err != nil {
				wg.Done()
				return err
			}
			mutex.Lock()
			result = append(result, Notification{Reason: r, URL: url, Title: t})
			mutex.Unlock()
			wg.Done()
			return nil
		}(n)
	}
	wg.Wait()
	return result, nil
}

func repoValidator(includes, excludes []string, repo string) bool {
	// both empty
	if len(includes) == 0 && len(excludes) == 0 {
		return true
	}
	// only includes
	if len(includes) > 0 && len(excludes) == 0 {
		for _, i := range includes {
			if i == repo {
				return true
			}
		}
		return false
	}
	// only excludes
	if len(includes) == 0 && len(excludes) > 0 {
		for _, e := range excludes {
			if e == repo {
				return false
			}
		}
		return true
	}
	// both are present, includes has priority
	for _, i := range includes {
		if i == repo {
			return true
		}
	}
	for _, e := range excludes {
		if e == repo {
			return false
		}
	}
	return false
}

func request(url string, e *Env) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.NotificationGithubToken))
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
	resultURL := mapBody.(map[string]interface{})["html_url"].(string)
	return resultURL, nil
}

func newBlock(n *Notification) *slack.MsgOption {
	text := fmt.Sprintf(":bell: *%s Notification*\n<%s|%s>", n.Reason, n.URL, n.Title)
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
