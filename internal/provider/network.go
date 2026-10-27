// SPDX-License-Identifier: AGPL-3.0-only
package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Network struct {
	Client *http.Client
	hosts  map[string]bool
}

func NewNetwork(hosts []string) *Network {
	jar, _ := cookiejar.New(nil)
	n := &Network{hosts: map[string]bool{}, Client: &http.Client{Timeout: 20 * time.Second, Jar: jar}}
	for _, h := range hosts {
		n.hosts[h] = true
	}
	n.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || (len(via) > 0 && via[0].Method != "GET") {
			return errors.New("redirect rejected")
		}
		return n.validate(req.URL)
	}
	return n
}
func (n *Network) validate(u *url.URL) error {
	if u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") || !n.hosts[strings.ToLower(u.Hostname())] {
		return errors.New("不支持此请求地址")
	}
	return nil
}
func (n *Network) Do(ctx context.Context, method, raw string, body []byte, headers http.Header) ([]byte, error) {
	u, e := url.Parse(raw)
	if e != nil {
		return nil, errors.New("无效网址")
	}
	if e = n.validate(u); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if e != nil {
		return nil, errors.New("请求创建失败")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 KindleRead/0.1")
	for k, v := range headers {
		req.Header[k] = v
	}
	res, e := n.Client.Do(req)
	if e != nil {
		return nil, errors.New("网络请求失败，请检查网络、设备时间和证书；云环境还需放行平台域名")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("平台返回 HTTP %d；未自动重试", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if e != nil {
		return nil, errors.New("响应读取失败")
	}
	if len(b) > 4<<20 {
		return nil, errors.New("平台响应超过 4 MiB 限制")
	}
	return b, nil
}
