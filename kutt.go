package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

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

func (cli *Client) Submit(ctx context.Context, target string) (*URL, error) {
	reqURL := fmt.Sprintf("%s/%s", cli.BaseURL, "api/url/submit")

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

	var u URL
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("Parse HTTP body: %w", err)
	}

	return &u, nil
}

func (cli *Client) ReplaceLinks(ctx context.Context, target string) (error, string) {
	r := regexp.MustCompile(`(http[\w:+//.#?={}%]+)`)
	raw := r.FindAllString(target, -1)
	for _, r := range raw {
		url, err := cli.Submit(ctx, r)
		if err != nil {
			return err, target
		}
		clog.Infof("Shortened link: %d, to: %d", url.Target, url.ShortURL)
		strings.Replace(target, r, url.ShortURL, 1)
	}
	return nil, target
}
