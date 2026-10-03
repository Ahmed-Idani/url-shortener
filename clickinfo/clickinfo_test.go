package clickinfo_test

import (
	"testing"

	"github.com/Ahmed-Idani/clickinfo"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name      string
		userAgent string
		browser   string
		os        string
		device    string
	}{
		{
			name:      "chrome on windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			browser:   "Chrome",
			os:        "Windows",
			device:    "Desktop",
		},
		{
			name:      "safari on iphone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
			browser:   "Safari",
			os:        "iOS",
			device:    "Mobile",
		},
		{
			name:      "empty",
			userAgent: "",
			browser:   "",
			os:        "",
			device:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			browser, os, device := clickinfo.Parse(tc.userAgent)
			if browser != tc.browser || os != tc.os || device != tc.device {
				t.Errorf("expected: %q %q %q, got: %q %q %q", tc.browser, tc.os, tc.device, browser, os, device)
			}
		})
	}
}

func TestReferrerHost(t *testing.T) {
	testCases := []struct {
		referer  string
		expected string
	}{
		{"https://www.linkedin.com/feed/", "linkedin.com"},
		{"https://t.co/abc", "t.co"},
		{"HTTPS://News.YCombinator.com/item?id=1", "news.ycombinator.com"},
		{"", ""},
		{"not a url", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.referer, func(t *testing.T) {
			if got := clickinfo.ReferrerHost(tc.referer); got != tc.expected {
				t.Errorf("expected: %q, got: %q", tc.expected, got)
			}
		})
	}
}
