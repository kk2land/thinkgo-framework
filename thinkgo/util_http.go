package thinkgo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientIp 从请求头获取客户端ip
func ClientIp(r *http.Request) string {
	if r == nil {
		return "127.0.0.1"
	}
	val := r.Header.Get("X-Forwarded-For")
	if len(val) > 0 {
		vals := strings.Split(val, ",")
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if len(v) > 0 && v != "unknown" {
				return v
			}
		}
	} else {
		val = r.Header.Get("X-Real-IP")
		if len(val) > 0 {
			return val
		}
	}
	return r.RemoteAddr
}

// Values2JMap 将请求query参数转换成jmap
func Values2JMap(values url.Values) JMap {
	data := make(JMap)
	for k, v := range values {
		if len(v) == 1 {
			data[k] = v[0]
		} else if len(v) > 1 {
			data[k] = v
		}
	}
	return data
}

// ParseForm 解析form表单请求
func ParseForm(r *http.Request) (data JMap, err error) {
	ct := r.Header.Get("Content-Type")
	if ct == "application/json" {
		if r.Body == nil {
			err = errors.New("missing json body")
			return
		}
		var body []byte
		if body, err = ioutil.ReadAll(r.Body); err != nil {
			return
		} else {
			err = json.Unmarshal(body, &data)
		}
	} else if strings.HasPrefix(ct, "multipart/form-data") {
		err = r.ParseMultipartForm(HttpEngine().MaxMultipartMemory)
		if err != nil {
			return
		}
		data = Values2JMap(r.MultipartForm.Value)
	} else {
		var f = ct != "application/x-www-form-urlencoded"
		if f {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if err = r.ParseForm(); err != nil {
			return
		}
		data = Values2JMap(r.PostForm)
		if f {
			r.Header.Set("Content-Type", ct)
		}
	}
	return
}

// HttpGet 简化http-get请求
func HttpGet(url string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return -1, nil, err
	}
	return HttpBody(req)
}

// HttpPost 简化http-post请求
func HttpPost(url, contentType string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return -1, nil, err
	}
	return HttpBody(req)
}

// HttpPostForm 简化http-post提交form表单格式请求
func HttpPostForm(url string, data url.Values) (int, []byte, error) {
	return HttpPost(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// HttpBody 发起http请求，需要返回body内容
func HttpBody(req *http.Request) (int, []byte, error) {
	return HttpBodyWithTries(req, 5*time.Second, BackoffPolicyDefault(time.Millisecond, 1))
}

// HttpBodyWithTries 发起http请求，需要返回body内容，支持重试
func HttpBodyWithTries(req *http.Request, timeout time.Duration, backoff BackoffPolicy) (code int, body []byte, err error) {
	read := func(req *http.Request) (int, []byte, error) {
		resp, err1 := http.DefaultClient.Do(req)
		if err1 != nil {
			return -1, nil, err1
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		var body1 []byte
		if body1, err1 = ioutil.ReadAll(resp.Body); err1 != nil {
			return -1, nil, err1
		}
		return resp.StatusCode, body1, nil
	}

	for backoff.Next() {
		code, body, err = func() (int, []byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return read(req.WithContext(ctx))
		}()
		if err == nil {
			return
		} else if backoff.End() {
			break
		}
		time.Sleep(backoff.Get())
	}
	return
}
