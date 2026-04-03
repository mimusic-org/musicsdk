//go:build wasip1

package musicsdk

import (
	"fmt"
	"io"
	"strings"

	"github.com/mimusic-org/plugin/pkg/go-plugin-http/http"
)

// DefaultUserAgent 默认 User-Agent
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

// HTTPGet 发送 GET 请求并返回响应体字节
func HTTPGet(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置默认 User-Agent
	req.Header.Set("User-Agent", DefaultUserAgent)

	// 设置自定义请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// HTTPPost 发送 POST 请求（支持 JSON body 或 form data）
func HTTPPost(url string, body []byte, headers map[string]string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置默认 User-Agent
	req.Header.Set("User-Agent", DefaultUserAgent)

	// 设置自定义请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// HTTPPostJSON 发送 POST 请求，自动设置 Content-Type 为 application/json
func HTTPPostJSON(url string, body []byte, headers map[string]string) ([]byte, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/json"
	return HTTPPost(url, body, headers)
}

// HTTPPostForm 发送 POST 请求，自动设置 Content-Type 为 application/x-www-form-urlencoded
func HTTPPostForm(url string, body []byte, headers map[string]string) ([]byte, error) {
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Content-Type"] = "application/x-www-form-urlencoded"
	return HTTPPost(url, body, headers)
}
