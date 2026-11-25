package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("server rejected agent token (401)")

const maxJobSize = 64 << 20

type Job struct {
	ID            string `json:"id"`
	Format        string `json:"format"`
	Copies        int    `json:"copies"`
	Printer       string `json:"printer"`
	PayloadBase64 string `json:"payload_base64"`
}

type Ack struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type Heartbeat struct {
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Printer  string `json:"printer"`
}

type Options struct {
	BaseURL     string
	Token       string
	UserAgent   string
	InsecureTLS bool
}

type Client struct {
	base      *url.URL
	token     string
	userAgent string
	http      *http.Client
}

func New(opts Options) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(opts.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse server url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("server url must start with https:// or http://, got %q", opts.BaseURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("server url has no host: %q", opts.BaseURL)
	}
	if opts.Token == "" {
		return nil, errors.New("agent token is empty")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		base:      u,
		token:     opts.Token,
		userAgent: opts.UserAgent,
		http:      &http.Client{Transport: transport},
	}, nil
}

func (c *Client) NextJob(ctx context.Context, wait time.Duration) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, wait+15*time.Second)
	defer cancel()

	q := url.Values{"wait": {strconv.Itoa(int(wait / time.Second))}}
	resp, err := c.do(ctx, http.MethodGet, "/api/agent/jobs/next?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer drain(resp)

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, nil
	case http.StatusOK:
	default:
		return nil, statusError(resp)
	}

	var job Job
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJobSize)).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode job: %w", err)
	}
	if job.ID == "" {
		return nil, errors.New("server returned job without id")
	}
	return &job, nil
}

func (c *Client) Ack(ctx context.Context, jobID string, ack Ack) error {
	return c.post(ctx, "/api/agent/jobs/"+url.PathEscape(jobID)+"/ack", ack)
}

func (c *Client) Heartbeat(ctx context.Context, hb Heartbeat) error {
	return c.post(ctx, "/api/agent/heartbeat", hb)
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPost, path, data)
	if err != nil {
		return err
	}
	defer drain(resp)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return statusError(resp)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, pathAndQuery string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+pathAndQuery, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return c.http.Do(req)
}

func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}

func drain(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
}
