// Modified or added for XiaoSeek LX04, 2026-10-24. See NOTICE.
// SPDX-License-Identifier: MIT
package miservice

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAPICallUsesOnlyRequestedServiceToken(t *testing.T) {
	a := NewAccount("unused", "unused", nil)
	a.token = NewTokens()
	a.token.UserId = "test"
	a.token.Sids["micoapi"] = SidToken{ServiceToken: "correct"}
	u, _ := url.Parse("https://userprofile.mina.mi.com/")
	a.client.Jar.SetCookies(u, []*http.Cookie{{Name: "serviceToken", Value: "another-service"}})
	a.client.Transport = otpTransport(func(r *http.Request) (*http.Response, error) {
		count := 0
		for _, c := range r.Cookies() {
			if c.Name == "serviceToken" {
				count++
				if c.Value != "correct" {
					t.Fatal("another SID's cookie was sent")
				}
			}
		}
		if count != 1 {
			t.Fatalf("got %d serviceToken cookies", count)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":0}`)), Request: r}, nil
	})
	var out map[string]any
	if err := a.Request("micoapi", u.String(), nil, nil, nil, false, &out); err != nil {
		t.Fatal(err)
	}
}

func TestXiaomiRateLimitIncludesRetryAfterWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		name, retryAfter, want string
	}{
		{"with header", "120", "Retry-After \"120\""},
		{"without header", "", "Retry-After absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAccount("unused", "unused", nil)
			a.token = NewTokens()
			a.token.UserId = "test"
			a.token.Sids["xiaomiio"] = SidToken{ServiceToken: "test"}
			calls := 0
			a.client.Transport = otpTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				h := make(http.Header)
				if tc.retryAfter != "" {
					h.Set("Retry-After", tc.retryAfter)
				}
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: h,
					Body: io.NopCloser(strings.NewReader("limited")),
					Request: r,
				}, nil
			})
			var out map[string]any
			err := a.Request("xiaomiio", "https://api.io.mi.com/app/miotspec/action", nil, nil, nil, true, &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 1 {
				t.Fatalf("HTTP 429 must not retry: got %d requests", calls)
			}
		})
	}
}
