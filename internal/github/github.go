// Package github calls the GitHub REST API.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Notification is a notification thread.
type Notification struct {
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updated_at"`
	Subject   struct {
		Type string `json:"type"`
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// Issue is an issue or pull request from the search API.
type Issue struct {
	HTMLURL       string    `json:"html_url"`
	RepositoryURL string    `json:"repository_url"`
	Number        int       `json:"number"`
	PullRequest   *struct{} `json:"pull_request"`
}

// Repo returns OWNER/REPO.
func (i Issue) Repo() string {
	p := strings.Split(strings.TrimRight(i.RepositoryURL, "/"), "/")
	if len(p) < 2 {
		return ""
	}
	return p[len(p)-2] + "/" + p[len(p)-1]
}

// Client wraps GitHub API operations.
type Client struct {
	http *http.Client
	rest *api.RESTClient
}

// NewClient returns a client authenticated as the gh user.
func NewClient() (*Client, error) {
	// RESTClient treats 304 as an error, so notifications use the plain HTTP client.
	h, err := api.DefaultHTTPClient()
	if err != nil {
		return nil, err
	}
	r, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return &Client{http: h, rest: r}, nil
}

// all=true includes notifications already read (e.g. by opening the email).
// participating=true excludes repositories that are only watched.
const notificationsURL = "https://api.github.com/notifications?all=true&participating=true&per_page=50"

// Notifications fetches notifications with If-None-Match. A 304 does not count against the rate limit.
func (c *Client) Notifications(etag string) ([]Notification, string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, notificationsURL, nil)
	if err != nil {
		return nil, "", false, err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, true, nil
	case http.StatusOK:
	default:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", false, fmt.Errorf("notifications: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var ns []Notification
	if err := json.NewDecoder(resp.Body).Decode(&ns); err != nil {
		return nil, "", false, err
	}
	return ns, resp.Header.Get("ETag"), false, nil
}

// SearchIssues returns up to 1000 issues and pull requests matching query.
func (c *Client) SearchIssues(query string) ([]Issue, error) {
	var all []Issue
	for page := 1; page <= 10; page++ {
		var res struct {
			Items []Issue `json:"items"`
		}
		path := fmt.Sprintf("search/issues?per_page=100&page=%d&q=%s", page, url.QueryEscape(query))
		if err := c.rest.Get(path, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Items...)
		if len(res.Items) < 100 {
			break
		}
	}
	return all, nil
}

// PullRequestHead returns the head SHA of a pull request.
func (c *Client) PullRequestHead(repo string, number int) (string, error) {
	var res struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := c.rest.Get(fmt.Sprintf("repos/%s/pulls/%d", repo, number), &res); err != nil {
		return "", err
	}
	return res.Head.SHA, nil
}
