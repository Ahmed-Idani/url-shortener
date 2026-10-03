package bot

import (
	"github.com/Ahmed-Idani/db_ops"
	"github.com/x-way/crawlerdetect"
)

func Origin(userAgent string) int {
	if userAgent == "" || crawlerdetect.IsCrawler(userAgent) {
		return db_ops.OriginBot
	}
	return db_ops.OriginUnknown
}
