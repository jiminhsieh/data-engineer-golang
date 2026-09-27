package weather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultMaxResponseBytes int64 = 1 << 20

type Result struct {
	Raw     []byte
	Weather Current
}

type Client struct {
	baseURL, apiKey     string
	latitude, longitude float64
	http                *http.Client
	maxBytes            int64
}

func NewClient(baseURL, apiKey string, latitude, longitude float64, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, latitude: latitude, longitude: longitude, http: &http.Client{Timeout: timeout}, maxBytes: DefaultMaxResponseBytes}
}

func NewClientWithHTTP(baseURL, apiKey string, latitude, longitude float64, client *http.Client, maxBytes int64) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, latitude: latitude, longitude: longitude, http: client, maxBytes: maxBytes}
}

func (c *Client) Fetch(ctx context.Context) (Result, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return Result{}, errors.New("build weather request: invalid endpoint")
	}
	q := u.Query()
	q.Set("lat", strconv.FormatFloat(c.latitude, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(c.longitude, 'f', -1, 64))
	q.Set("appid", c.apiKey)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, errors.New("build weather request")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("weather request failed: %w", redactError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Result{}, fmt.Errorf("weather request returned status %d", resp.StatusCode)
	}
	limit := c.maxBytes
	if limit <= 0 {
		limit = DefaultMaxResponseBytes
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Result{}, errors.New("read weather response")
	}
	if int64(len(raw)) > limit {
		return Result{}, errors.New("weather response exceeds size limit")
	}
	decoded, err := Decode(raw)
	if err != nil {
		return Result{}, err
	}
	return Result{Raw: raw, Weather: decoded}, nil
}

type safeError struct{ kind string }

func (e safeError) Error() string { return e.kind }
func redactError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return safeError{kind: "timeout"}
	}
	if errors.Is(err, context.Canceled) {
		return safeError{kind: "canceled"}
	}
	return safeError{kind: "network error"}
}
