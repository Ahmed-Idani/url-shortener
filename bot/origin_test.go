package bot_test

import (
	"testing"

	"github.com/Ahmed-Idani/bot"
	"github.com/Ahmed-Idani/db_ops"
)

func TestOrigin(t *testing.T) {
	testCases := []struct {
		name      string
		userAgent string
		expected  int
	}{
		{
			name:      "chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			expected:  db_ops.OriginUnknown,
		},
		{
			name:      "googlebot",
			userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			expected:  db_ops.OriginBot,
		},
		{
			name:      "linkedin",
			userAgent: "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)",
			expected:  db_ops.OriginBot,
		},
		{
			name:      "whatsapp preview",
			userAgent: "WhatsApp/2.23.20.0",
			expected:  db_ops.OriginBot,
		},
		{
			name:      "curl",
			userAgent: "curl/8.5.0",
			expected:  db_ops.OriginBot,
		},
		{
			name:      "empty",
			userAgent: "",
			expected:  db_ops.OriginBot,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := bot.Origin(tc.userAgent)
			if result != tc.expected {
				t.Errorf("expected: %d, got: %d", tc.expected, result)
			}
		})
	}
}
