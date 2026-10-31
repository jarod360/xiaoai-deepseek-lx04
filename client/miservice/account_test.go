// Modified or added for XiaoSeek LX04, 2026-10-24. See NOTICE.
// SPDX-License-Identifier: MIT
package miservice

import (
	"context"
	"errors"
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

func TestXiaomiLoginRateLimitSkipsPasswordAuthentication(t *testing.T) {
	a := NewAccount("unused", "unused", nil)
	calls := 0
	a.client.Transport = otpTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/pass/serviceLogin" {
			t.Fatalf("unexpected extra login request: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("&&&START&&&{\"code\":70022}")), Request: r}, nil
	})
	err := a.Login("xiaomiio")
	if err == nil || !strings.Contains(err.Error(), "70022") {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one login request, got %d", calls)
	}
}

func TestXiaomiRequestContextCancellation(t *testing.T) {
	a := NewAccount("unused", "unused", nil)
	a.token = NewTokens()
	a.token.UserId = "test"
	a.token.Sids["xiaomiio"] = SidToken{ServiceToken: "test"}
	called := false
	a.client.Transport = otpTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{\"code\":0}")), Request: r}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out map[string]any
	err := a.RequestContext(ctx, "xiaomiio", "https://api.io.mi.com/app/miotspec/action", nil, nil, nil, true, &out)
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("cancelled request was sent: %v (called=%v)", err, called)
	}
}
