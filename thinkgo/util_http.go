package thinkgo

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func HttpGet(url string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return -1, nil, err
	}
	var resp *http.Response
	if resp, err = HttpRequest(req); err != nil {
		return -1, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return -1, nil, err
	}
	return resp.StatusCode, body, nil
}

func HttpPost(url, contentType string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return -1, nil, err
	}
	req.Header.Set("Content-Type", contentType)
	var resp *http.Response
	if resp, err = HttpRequest(req); err != nil {
		return -1, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	var rbody []byte
	if rbody, err = ioutil.ReadAll(resp.Body); err != nil {
		return -1, nil, err
	}
	return resp.StatusCode, rbody, nil
}

func HttpPostForm(url string, data url.Values) (int, []byte, error) {
	return HttpPost(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func HttpBody(req *http.Request) (int, []byte, error) {
	return HttpBodyWithTries(req, 5*time.Second, BackoffPolicyDefault(time.Millisecond, 1))
}

func HttpBodyWithTries(req *http.Request, timeout time.Duration, backoff BackoffPolicy) (int, []byte, error) {
	resp, err := HttpRequestWithTries(req, timeout, backoff)
	if err != nil {
		return -1, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	var rbody []byte
	if rbody, err = ioutil.ReadAll(resp.Body); err != nil {
		return -1, nil, err
	}
	return resp.StatusCode, rbody, nil
}

func HttpRequest(req *http.Request) (*http.Response, error) {
	return HttpRequestWithTries(req, 5*time.Second, BackoffPolicyDefault(time.Millisecond, 1))
}

func HttpRequestWithTries(req *http.Request, timeout time.Duration, backoff BackoffPolicy) (*http.Response, error) {
	var resp *http.Response
	var err error
	for backoff.Next() {
		resp, err = func() (*http.Response, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return http.DefaultClient.Do(req.WithContext(ctx))
		}()
		if err == nil {
			return resp, nil
		} else if backoff.End() {
			break
		}
		time.Sleep(backoff.Get())
	}
	return nil, err
}
