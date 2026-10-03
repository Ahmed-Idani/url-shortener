package clickinfo

import (
	"net/url"
	"strings"

	"github.com/mileusna/useragent"
)

func deviceType(ua useragent.UserAgent) string {
	switch {
	case ua.Bot:
		return "Bot"
	case ua.Tablet:
		return "Tablet"
	case ua.Mobile:
		return "Mobile"
	case ua.Desktop:
		return "Desktop"
	default:
		return ""
	}
}

func Parse(userAgent string) (browser, os, device string) {
	ua := useragent.Parse(userAgent)
	return ua.Name, ua.OS, deviceType(ua)
}

func ReferrerHost(referer string) string {
	u, err := url.Parse(referer)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
