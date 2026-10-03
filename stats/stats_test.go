package stats

import (
	"testing"
	"time"

	"github.com/Ahmed-Idani/db_ops"
)

var tunis = mustLoad("Africa/Tunis")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func click(t time.Time, ip string, origin int) db_ops.Click {
	return db_ops.Click{Time: t.UTC(), IP: ip, Origin: origin, Location: "FR", Browser: "Chrome", OS: "Windows", Device: "Desktop"}
}

func TestNiceScale(t *testing.T) {
	testCases := []struct {
		max, step, top int
	}{
		{0, 1, 1},
		{3, 1, 3},
		{5, 1, 5},
		{7, 2, 8},
		{26, 10, 30},
		{438, 200, 600},
		{1000, 500, 1000},
	}
	for _, tc := range testCases {
		step, top := niceScale(tc.max)
		if step != tc.step || top != tc.top {
			t.Errorf("niceScale(%d): expected %d/%d, got %d/%d", tc.max, tc.step, tc.top, step, top)
		}
	}
}

func TestRangeStart(t *testing.T) {
	now := time.Date(2026, 10, 3, 14, 30, 0, 0, tunis)

	if got, want := FindRange("7d").Start(now, tunis), time.Date(2026, 9, 27, 0, 0, 0, 0, tunis); !got.Equal(want) {
		t.Errorf("7d start: expected %v, got %v", want, got)
	}
	if got, want := FindRange("7d").PreviousStart(now, tunis), time.Date(2026, 9, 20, 0, 0, 0, 0, tunis); !got.Equal(want) {
		t.Errorf("7d previous start: expected %v, got %v", want, got)
	}
	if got, want := FindRange("24h").Start(now, tunis), time.Date(2026, 10, 2, 15, 0, 0, 0, tunis); !got.Equal(want) {
		t.Errorf("24h start: expected %v, got %v", want, got)
	}
	if !FindRange("all").Start(now, tunis).IsZero() {
		t.Errorf("all start should be zero")
	}
	if FindRange("nope").Key != "30d" {
		t.Errorf("unknown range should default to 30d")
	}
}

func TestBuild(t *testing.T) {
	now := time.Date(2026, 10, 3, 14, 30, 0, 0, tunis)
	clicks := []db_ops.Click{
		click(time.Date(2026, 9, 21, 10, 0, 0, 0, tunis), "9.9.9.9", db_ops.OriginUnknown), // previous period
		click(time.Date(2026, 9, 27, 0, 30, 0, 0, tunis), "1.1.1.1", db_ops.OriginUnknown),
		click(time.Date(2026, 10, 3, 9, 0, 0, 0, tunis), "1.1.1.1", db_ops.OriginUnknown),
		click(time.Date(2026, 10, 3, 9, 5, 0, 0, tunis), "2.2.2.2", db_ops.OriginUnknown),
		click(time.Date(2026, 10, 3, 9, 10, 0, 0, tunis), "3.3.3.3", db_ops.OriginBot),
	}

	d := Build(clicks, Options{Range: FindRange("7d"), Now: now, Loc: tunis})

	if d.Visitors != 3 || d.UniqueVisitors != 2 || d.Bots != 1 || d.BotShare != 25 {
		t.Errorf("totals: got visitors=%d unique=%d bots=%d share=%d", d.Visitors, d.UniqueVisitors, d.Bots, d.BotShare)
	}
	if d.VisitorsDelta == nil || !d.VisitorsDelta.Up || d.VisitorsDelta.Percent != 200 {
		t.Errorf("visitors delta: got %+v", d.VisitorsDelta)
	}
	if d.Shown != 3 {
		t.Errorf("bots excluded: expected 3 shown, got %d", d.Shown)
	}
	if n := len(d.Timeline.Buckets); n != 7 {
		t.Fatalf("expected 7 daily buckets, got %d", n)
	}
	// 00:30 in Tunis is still Sep 26 in UTC: it must land in the first local day.
	if first := d.Timeline.Buckets[0]; first.Visitors != 1 || first.Title != "Sun, Sep 27" {
		t.Errorf("first bucket: got %+v", first)
	}
	if last := d.Timeline.Buckets[6]; last.Visitors != 2 || last.Bots != 0 {
		t.Errorf("last bucket: got %+v", last)
	}
	if cell := d.Heatmap.Days[5].Cells[9]; d.Heatmap.Days[5].Name != "Sat" || cell.Count != 2 || cell.Level != 5 {
		t.Errorf("saturday 09:00 cell: got %+v", cell)
	}
	if len(d.Recent) != 3 || d.Recent[0].IP != "2.2.2.2" {
		t.Errorf("recent should be newest first without bots: got %+v", d.Recent)
	}

	withBots := Build(clicks, Options{Range: FindRange("7d"), IncludeBots: true, Now: now, Loc: tunis})
	if last := withBots.Timeline.Buckets[6]; last.Bots != 1 {
		t.Errorf("bots included: last bucket got %+v", last)
	}
	if !withBots.Recent[0].Bot {
		t.Errorf("bots included: newest click should be the bot")
	}
}

func TestNewDelta(t *testing.T) {
	testCases := []struct {
		current, previous int
		want              *Delta
	}{
		{0, 0, nil},
		{5, 0, &Delta{New: true, Up: true}},
		{10, 10, &Delta{Flat: true}},
		{15, 10, &Delta{Percent: 50, Up: true}},
		{5, 10, &Delta{Percent: 50}},
	}
	for _, tc := range testCases {
		got := newDelta(tc.current, tc.previous)
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Errorf("newDelta(%d, %d): expected %+v, got %+v", tc.current, tc.previous, tc.want, got)
		}
	}
}

func TestTopRowsFoldsOther(t *testing.T) {
	var clicks []db_ops.Click
	for i, code := range []string{"FR", "FR", "FR", "US", "US", "DE", "MA", "GB"} {
		c := click(time.Unix(int64(i), 0), "1.1.1.1", db_ops.OriginUnknown)
		c.Location = code
		clicks = append(clicks, c)
	}
	rows := topRows(clicks, 3, func(c db_ops.Click) string { return c.Location })
	if len(rows) != 3 || rows[0].Label != "FR" || rows[1].Label != "US" || rows[2].Label != "Other" || rows[2].Count != 3 {
		t.Errorf("got %+v", rows)
	}
	if rows[0].Percent != 38 || rows[0].Width != 100 {
		t.Errorf("first row: got %+v", rows[0])
	}
}

func TestCountryLabel(t *testing.T) {
	testCases := map[string]string{
		"FR": "🇫🇷 France",
		"ma": "🇲🇦 Morocco",
		"ZZ": "Unknown",
		"":   "Unknown",
	}
	for code, want := range testCases {
		if got := CountryLabel(code); got != want {
			t.Errorf("CountryLabel(%q): expected %q, got %q", code, want, got)
		}
	}
}
