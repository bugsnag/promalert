package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"mvdan.cc/xurls/v2"

	"github.com/bugsnag/microkit/clog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Client struct {
	HTTPClient *http.Client
	ApiKey     string
	BaseURL    string
	UserAgent  string
}

type SubmitParams struct {
	URL      string `json:"target"`
	ExpireIn string `json:"expire_in"`
}

type LinkResponse struct {
	Address     string    `json:"address"`
	Banned      bool      `json:"banned"`
	CreatedAt   time.Time `json:"created_at"`
	ID          string    `json:"id"`
	Link        string    `json:"link"`
	Password    bool      `json:"password"`
	Target      string    `json:"target"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
	VisitCount  int       `json:"visit_count"`
}

func NewLinksClient() *Client {
	var cli Client
	cli.ApiKey = viper.GetString("kutt_api_key")
	cli.BaseURL = viper.GetString("kutt_base_url")
	cli.UserAgent = "promalert/v1 (+https://github.com/bugsnag/promalert)"
	cli.HTTPClient = &http.Client{
		Timeout: time.Second * 10,
	}
	return &cli
}

func (cli *Client) error(statusCode int, body io.Reader) error {
	buf, err := ioutil.ReadAll(body)
	if err != nil || len(buf) == 0 {
		return errors.Errorf("Request failed with status code %d", statusCode)
	}
	return errors.Errorf("StatusCode: %d, Error: %s", statusCode, string(buf))
}

func (cli *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-API-Key", cli.ApiKey)
	req.Header.Set("User-Agent", cli.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return cli.HTTPClient.Do(req)
}

func (cli *Client) Submit(ctx context.Context, target string) (*LinkResponse, error) {
	reqURL := fmt.Sprintf("%s/%s", cli.BaseURL, "api/v2/links")

	payload := &SubmitParams{
		URL:      target,
		ExpireIn: viper.GetString("kutt_link_expiry"),
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Marshal json: %w", err)
	}

	body := strings.NewReader(string(jsonBytes))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("Create HTTP request: %w", err)
	}

	resp, err := cli.do(req)
	if err != nil {
		return nil, fmt.Errorf("Do HTTP request: %w", err)
	}

	defer resp.Body.Close()

	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return nil, fmt.Errorf("HTTP response: %w", cli.error(resp.StatusCode, resp.Body))
	}

	var u LinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("Parse HTTP body: %w", err)
	}

	return &u, nil
}

func (cli *Client) ReplaceLinks(ctx context.Context, target string) (error, string) {
	r := xurls.Strict()
	raw := r.FindAllString(target, -1)
	for _, r := range raw {
		url, err := cli.Submit(ctx, r)
		if err != nil {
			return err, target
		}
		clog.Infof("Shortened link: %s, to: %s", url.Target, url.Link)
		target = strings.Replace(target, r, url.Link, 1)
	}
	return nil, target
}
