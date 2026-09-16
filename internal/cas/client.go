package cas

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	ServerURL  string
	ServiceURL string
	HTTP       *http.Client
}

func New(serverURL, serviceURL string) *Client {
	return &Client{
		ServerURL:  strings.TrimRight(serverURL, "/"),
		ServiceURL: serviceURL,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
	}
}

// LoginURL 返回 CAS 登录跳转地址；ServerURL 为空时返回空串。
func (c *Client) LoginURL() string {
	if c.ServerURL == "" {
		return ""
	}
	q := url.Values{}
	q.Set("service", c.ServiceURL)
	return c.ServerURL + "/login?" + q.Encode()
}

// LogoutURL 返回 CAS 登出地址；ServerURL 为空时返回空串。
func (c *Client) LogoutURL() string {
	if c.ServerURL == "" {
		return ""
	}
	q := url.Values{}
	q.Set("service", c.ServiceURL)
	return c.ServerURL + "/logout?" + q.Encode()
}

type serviceResponse struct {
	XMLName xml.Name `xml:"serviceResponse"`
	Success *struct {
		User string `xml:"user"`
	} `xml:"authenticationSuccess"`
	Failure *struct {
		Code    string `xml:"code,attr"`
		Message string `xml:"message"`
	} `xml:"authenticationFailure"`
}

// Validate 校验 CAS ticket，返回学号。
func (c *Client) Validate(ctx context.Context, ticket string) (string, error) {
	if c.ServerURL == "" {
		return "", fmt.Errorf("CAS_SERVER_URL 未配置")
	}
	q := url.Values{}
	q.Set("service", c.ServiceURL)
	q.Set("ticket", ticket)
	endpoint := c.ServerURL + "/serviceValidate?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("CAS HTTP %d: %s", resp.StatusCode, string(body))
	}

	var sr serviceResponse
	if err := xml.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return "", fmt.Errorf("解析 CAS 响应: %w", err)
	}
	if sr.Failure != nil {
		return "", fmt.Errorf("CAS 认证失败: %s %s", sr.Failure.Code, sr.Failure.Message)
	}
	if sr.Success == nil || strings.TrimSpace(sr.Success.User) == "" {
		return "", fmt.Errorf("CAS 响应中没有 user")
	}
	return strings.TrimSpace(sr.Success.User), nil
}
