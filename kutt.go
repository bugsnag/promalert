import (
	"io"
	"io/ioutil"
	"strings"
	"net/http"
	"encoding/json"
	"regexp"
)

type Client struct {
	HTTPClient *http.Client
	ApiKey     string
	BaseURL    string
	UserAgent  string
}

func NewClient() *Client {
	var cli Client
	cli.ApiKey = viper.GetString("kutt_api_key")
	cli.BaseURL = viper.GetString("kutt_base_url")
	cli.UserAgent = "promalert/v1 (+https://github.com/bugsnag/promalert)"

	return &cli
}

func (cli *Client) error(statusCode int, body io.Reader) error {
	buf, err := ioutil.ReadAll(body)
	if err != nil || len(buf) == 0 {
		return errors.Errorf("request failed with status code %d", statusCode)
	}
	return errors.Errorf("StatusCode: %d, Error: %s", statusCode, string(buf))
}

func (cli *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-API-Key", cli.ApiKey)
	req.Header.Set("User-Agent", cli.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return cli.httpClient().Do(req)
}

func (cli *Client) Submit(target string) (*URL, error) {
	path := "/api/url/submit"
	reqURL := cli.BaseURL + path

	payload := &SubmitParams{
		URL: target,
		expire_in: "365d"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	body := strings.NewReader(string(jsonBytes))
	req, err := http.NewRequest(http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}

	resp, err := cli.do(req)
	if err != nil {
		return nil, fmt.Errorf("do HTTP request: %w", err)
	}

	defer resp.Body.Close()

	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return nil, fmt.Errorf("HTTP response: %w", cli.error(resp.StatusCode, resp.Body))
	}

	var u URL
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("parse HTTP body: %w", err)
	}

	return &u, nil
}

func (cli *Client) ReplaceLinks(target string) error, string {
	r, _ := regexp.MustCompile(`(http[\w:+//.#?={}%]+)`)
	raw := r.FindAllString(target, -1)
	for _, r := range raw {
		p, err = cli.Submit(s)
		if err != nil {
			return target, err
		}
		strings.Replace(target, r, p, 1)
	}
	return target, nil
}
