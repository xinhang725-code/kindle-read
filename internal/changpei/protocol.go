// SPDX-License-Identifier: AGPL-3.0-only
package changpei

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/xinhang725-code/kindle-read/internal/provider"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

const base = "https://www.gongzicp.com"

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *Error) Error() string { return e.Message }
func problem(code, message string) *Error {
	return &Error{Code: code, Message: message, Retryable: code == "UPSTREAM_ERROR"}
}

var malformed = problem("UPSTREAM_ERROR", "长佩响应格式已变化，请稍后重试或更新应用")

// Only the small, published UserExternal protobuf messages are needed here.
// Unknown fields are skipped; bounds, wire types and UTF-8 are checked before use.
type field struct {
	wire   uint64
	data   []byte
	number uint64
}
type message map[uint64]field

func parse(b []byte) (message, error) {
	if len(b) > 1<<20 {
		return nil, malformed
	}
	m := message{}
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 || tag>>3 == 0 || tag>>3 > 0x1fffffff {
			return nil, malformed
		}
		b = b[n:]
		f := field{wire: tag & 7}
		switch f.wire {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, malformed
			}
			f.number = v
			b = b[n:]
		case 1:
			if len(b) < 8 {
				return nil, malformed
			}
			b = b[8:]
		case 2:
			l, n := binary.Uvarint(b)
			if n <= 0 || l > uint64(len(b)-n) {
				return nil, malformed
			}
			b = b[n:]
			f.data = b[:int(l)]
			b = b[int(l):]
		case 5:
			if len(b) < 4 {
				return nil, malformed
			}
			b = b[4:]
		default:
			return nil, malformed
		}
		if _, exists := m[tag>>3]; exists {
			return nil, malformed
		}
		m[tag>>3] = f
	}
	return m, nil
}
func (m message) text(n uint64) string {
	f := m[n]
	if f.wire != 2 || !utf8.Valid(f.data) {
		return ""
	}
	return string(f.data)
}
func (m message) uint(n uint64) uint64 {
	f := m[n]
	if f.wire != 0 {
		return 0
	}
	return f.number
}
func (m message) nested(n uint64) (message, error) {
	f, ok := m[n]
	if !ok || f.wire != 2 {
		return nil, malformed
	}
	return parse(f.data)
}
func stringField(n uint64, s string) []byte {
	b := binary.AppendUvarint(nil, n<<3|2)
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, []byte(s)...)
}
func rpcStatus(s string) error {
	if s == "" || s == "0" {
		return nil
	}
	if s == "16" {
		return problem("AUTH_EXPIRED", "登录已过期，请重新扫码")
	}
	if s == "8" {
		return problem("RATE_LIMITED", "长佩请求过于频繁，请 60 秒后重试")
	}
	if _, e := strconv.Atoi(s); e != nil {
		return malformed
	}
	return problem("UPSTREAM_ERROR", "长佩暂未完成请求，请稍后重试（状态 "+s+"）")
}
func (c *Client) rpc(ctx context.Context, method string, payload []byte, token string) (message, error) {
	h := http.Header{}
	h.Set("Content-Type", "application/grpc-web+proto")
	h.Set("X-Grpc-Web", "1")
	h.Set("Version", "1")
	h.Set("Imei", c.fingerprint)
	h.Set("_c_n_", c.fingerprint)
	h.Set("Referer", base+"/login/signin")
	h.Set("Origin", base)
	if token != "" {
		h.Set("Token", token)
	}
	body := append(binary.BigEndian.AppendUint32([]byte{0}, uint32(len(payload))), payload...)
	b, headers, e := c.network.DoResponse(ctx, "POST", base+"/cp.UserExternal/"+method, body, h)
	if e != nil {
		return nil, asError(e)
	}
	if e = rpcStatus(headers.Get("Grpc-Status")); e != nil {
		return nil, e
	}
	var data []byte
	seen, trailers := false, false
	for len(b) > 0 {
		if len(b) < 5 {
			return nil, malformed
		}
		flag, l := b[0], binary.BigEndian.Uint32(b[1:5])
		b = b[5:]
		if uint64(l) > uint64(len(b)) {
			return nil, malformed
		}
		p := b[:int(l)]
		b = b[int(l):]
		switch flag {
		case 0:
			if seen || trailers {
				return nil, malformed
			}
			data = p
			seen = true
		case 128:
			if trailers || len(b) > 0 {
				return nil, malformed
			}
			trailers = true
			found := false
			for _, line := range strings.Split(string(p), "\r\n") {
				k, v, ok := strings.Cut(line, ":")
				if ok && strings.EqualFold(k, "grpc-status") {
					if found {
						return nil, malformed
					}
					found = true
					if e = rpcStatus(strings.TrimSpace(v)); e != nil {
						return nil, e
					}
				}
			}
			if !found {
				return nil, malformed
			}
		default:
			return nil, malformed
		}
	}
	// The official same-origin endpoint also returns a single application/grpc
	// data frame without grpc-web trailers (observed 2026-10-28).
	if !seen {
		return nil, malformed
	}
	return parse(data)
}
func validID(s string) bool {
	if len(s) == 0 || len(s) > 12 || s[0] == '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func asError(e error) *Error {
	var status *provider.HTTPError
	if errors.As(e, &status) {
		if status.StatusCode == 401 {
			return problem("AUTH_EXPIRED", "登录已过期，请重新扫码")
		}
		if status.StatusCode == 429 {
			return problem("RATE_LIMITED", "长佩请求过于频繁，请 60 秒后重试")
		}
	}
	var out *Error
	if errors.As(e, &out) {
		return out
	}
	return problem("UPSTREAM_ERROR", fmt.Sprint(e))
}
