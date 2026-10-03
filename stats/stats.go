package stats

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Ahmed-Idani/db_ops"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

type Range struct {
	Key   string
	Label string
	days  int
	hours int
}

var Ranges = []Range{
	{Key: "24h", Label: "24 hours", hours: 24},
	{Key: "7d", Label: "7 days", days: 7},
	{Key: "30d", Label: "30 days", days: 30},
	{Key: "90d", Label: "90 days", days: 90},
	{Key: "all", Label: "All time"},
}

func FindRange(key string) Range {
	for _, r := range Ranges {
		if r.Key == key {
			return r
		}
	}
	return Ranges[2]
}

// Start returns the first instant shown by the range, aligned to its buckets,
// so totals and the timeline always agree. Zero means all time.
func (r Range) Start(now time.Time, loc *time.Location) time.Time {
	now = now.In(loc)
	switch {
	case r.hours > 0:
		hour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
		return hour.Add(-time.Duration(r.hours-1) * time.Hour)
	case r.days > 0:
		return midnight(now).AddDate(0, 0, -(r.days - 1))
	default:
		return time.Time{}
	}
}

// PreviousStart returns the start of the equally long period before Start.
func (r Range) PreviousStart(now time.Time, loc *time.Location) time.Time {
	start := r.Start(now, loc)
	switch {
	case r.hours > 0:
		return start.Add(-time.Duration(r.hours) * time.Hour)
	case r.days > 0:
		return start.AddDate(0, 0, -r.days)
	default:
		return time.Time{}
	}
}

type Options struct {
	Range       Range
	IncludeBots bool
	Now         time.Time
	Loc         *time.Location
}

type Dashboard struct {
	Range       Range
	IncludeBots bool

	Visitors       int
	UniqueVisitors int
	Bots           int
	BotShare       int
	VisitorsDelta  *Delta
	UniqueDelta    *Delta
	LastVisit      string
	LastVisitTitle string

	Shown     int
	Timeline  Timeline
	Heatmap   Heatmap
	Countries []Row
	Referrers []Row
	Browsers  []Row
	Systems   []Row
	Devices   []Row
	Recent    []RecentClick
}

type Breakdown struct {
	Title string
	Rows  []Row
}

func (d Dashboard) Breakdowns() []Breakdown {
	return []Breakdown{
		{"Countries", d.Countries},
		{"Referrers", d.Referrers},
		{"Browsers", d.Browsers},
		{"Operating systems", d.Systems},
		{"Devices", d.Devices},
	}
}

type Delta struct {
	Percent int
	Up      bool
	Flat    bool
	New     bool
}

type Timeline struct {
	Buckets []Bucket
	Ticks   []Tick
	Unit    string
}

type Bucket struct {
	Title       string
	AxisLabel   string
	Visitors    int
	Bots        int
	VisitorsPct float64
	BotsPct     float64
	Tip         string
}

type Tick struct {
	Value int
	Pct   float64
}

type Heatmap struct {
	Days  []HeatDay
	Hours []string
	Max   int
}

type HeatDay struct {
	Name  string
	Cells []HeatCell
}

type HeatCell struct {
	Count int
	Level int
	Tip   string
}

type Row struct {
	Label   string
	Count   int
	Percent int
	Width   float64
}

type RecentClick struct {
	Time     string
	FullTime string
	Country  string
	Device   string
	Browser  string
	Referrer string
	IP       string
	Bot      bool
}

type tip struct {
	Title string     `json:"title"`
	Rows  [][]string `json:"rows"`
}

func tipJSON(title string, rows ...[]string) string {
	b, _ := json.Marshal(tip{Title: title, Rows: rows})
	return string(b)
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func isBot(c db_ops.Click) bool {
	return c.Origin == db_ops.OriginBot
}

// Build turns the clicks of one link, loaded from PreviousStart onwards, into the dashboard.
func Build(clicks []db_ops.Click, o Options) Dashboard {
	d := Dashboard{Range: o.Range, IncludeBots: o.IncludeBots}
	start := o.Range.Start(o.Now, o.Loc)
	prevStart := o.Range.PreviousStart(o.Now, o.Loc)

	var current, shown []db_ops.Click
	prevVisitors, prevIPs := 0, map[string]bool{}
	ips := map[string]bool{}
	for _, c := range clicks {
		if c.Time.Before(start) {
			if !c.Time.Before(prevStart) && !isBot(c) {
				prevVisitors++
				prevIPs[c.IP] = true
			}
			continue
		}
		current = append(current, c)
		if isBot(c) {
			d.Bots++
		} else {
			d.Visitors++
			ips[c.IP] = true
		}
		if o.IncludeBots || !isBot(c) {
			shown = append(shown, c)
		}
	}
	d.UniqueVisitors = len(ips)
	d.Shown = len(shown)
	if len(current) > 0 {
		d.BotShare = int(math.Round(float64(d.Bots) * 100 / float64(len(current))))
	}
	if !start.IsZero() {
		d.VisitorsDelta = newDelta(d.Visitors, prevVisitors)
		d.UniqueDelta = newDelta(d.UniqueVisitors, len(prevIPs))
	}
	for i := len(current) - 1; i >= 0; i-- {
		if !isBot(current[i]) {
			last := current[i].Time.In(o.Loc)
			d.LastVisit = relative(o.Now.Sub(last))
			d.LastVisitTitle = last.Format("Mon, Jan 2 2006 15:04")
			break
		}
	}

	d.Timeline = buildTimeline(shown, start, o)
	d.Heatmap = buildHeatmap(shown, o.Loc)
	d.Countries = topRows(shown, 8, func(c db_ops.Click) string { return CountryLabel(c.Location) })
	d.Referrers = topRows(shown, 8, func(c db_ops.Click) string { return orDefault(c.Referrer, "No referrer") })
	d.Browsers = topRows(shown, 6, func(c db_ops.Click) string { return orDefault(c.Browser, "Unknown") })
	d.Systems = topRows(shown, 6, func(c db_ops.Click) string { return orDefault(c.OS, "Unknown") })
	d.Devices = topRows(shown, 4, func(c db_ops.Click) string { return orDefault(c.Device, "Unknown") })

	for i := len(shown) - 1; i >= 0 && len(d.Recent) < 50; i-- {
		c := shown[i]
		t := c.Time.In(o.Loc)
		browser := orDefault(c.Browser, "Unknown")
		if c.OS != "" {
			browser += " · " + c.OS
		}
		d.Recent = append(d.Recent, RecentClick{
			Time:     t.Format("Jan 2, 15:04"),
			FullTime: t.Format("Mon, Jan 2 2006 15:04:05 MST"),
			Country:  CountryLabel(c.Location),
			Device:   orDefault(c.Device, "Unknown"),
			Browser:  browser,
			Referrer: orDefault(c.Referrer, "—"),
			IP:       c.IP,
			Bot:      isBot(c),
		})
	}
	return d
}

func newDelta(current, previous int) *Delta {
	if previous == 0 {
		if current == 0 {
			return nil
		}
		return &Delta{New: true, Up: true}
	}
	pct := int(math.Round(float64(current-previous) * 100 / float64(previous)))
	return &Delta{Percent: abs(pct), Up: pct > 0, Flat: pct == 0}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func relative(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func buildTimeline(clicks []db_ops.Click, start time.Time, o Options) Timeline {
	now := o.Now.In(o.Loc)
	t := Timeline{Unit: "day"}
	step := func(b time.Time) time.Time { return b.AddDate(0, 0, 1) }
	floor := midnight
	title := func(b time.Time) string { return b.Format("Mon, Jan 2") }
	axis := func(b time.Time) string { return b.Format("Jan 2") }

	switch {
	case o.Range.hours > 0:
		t.Unit = "hour"
		step = func(b time.Time) time.Time { return b.Add(time.Hour) }
		floor = func(b time.Time) time.Time {
			return time.Date(b.Year(), b.Month(), b.Day(), b.Hour(), 0, 0, 0, b.Location())
		}
		title = func(b time.Time) string {
			return b.Format("Mon, Jan 2 · 15:04") + "–" + b.Add(time.Hour).Format("15:04")
		}
		axis = func(b time.Time) string { return b.Format("15:04") }
	case start.IsZero():
		start = midnight(now)
		if len(clicks) > 0 {
			start = midnight(clicks[0].Time.In(o.Loc))
		}
		if now.Sub(start) > 120*24*time.Hour {
			t.Unit = "week"
			step = func(b time.Time) time.Time { return b.AddDate(0, 0, 7) }
			floor = func(b time.Time) time.Time {
				m := midnight(b)
				return m.AddDate(0, 0, -((int(m.Weekday()) + 6) % 7))
			}
			title = func(b time.Time) string { return "Week of " + b.Format("Mon, Jan 2 2006") }
			axis = func(b time.Time) string { return b.Format("Jan 2") }
			start = floor(start)
		}
	}

	index := map[int64]int{}
	for b := start.In(o.Loc); !b.After(now); b = step(b) {
		index[b.Unix()] = len(t.Buckets)
		t.Buckets = append(t.Buckets, Bucket{Title: title(b), AxisLabel: axis(b)})
	}
	for _, c := range clicks {
		i, ok := index[floor(c.Time.In(o.Loc)).Unix()]
		if !ok {
			continue
		}
		if isBot(c) {
			t.Buckets[i].Bots++
		} else {
			t.Buckets[i].Visitors++
		}
	}

	maxTotal := 0
	for _, b := range t.Buckets {
		maxTotal = max(maxTotal, b.Visitors+b.Bots)
	}
	tickStep, top := niceScale(maxTotal)
	for v := 0; v <= top; v += tickStep {
		t.Ticks = append(t.Ticks, Tick{Value: v, Pct: round2(float64(v) * 100 / float64(top))})
	}

	every := int(math.Ceil(float64(len(t.Buckets)) / 6))
	for i := range t.Buckets {
		b := &t.Buckets[i]
		b.VisitorsPct = round2(float64(b.Visitors) * 100 / float64(top))
		b.BotsPct = round2(float64(b.Bots) * 100 / float64(top))
		if (len(t.Buckets)-1-i)%every != 0 {
			b.AxisLabel = ""
		}
		rows := [][]string{{fmt.Sprint(b.Visitors), "Visitors", "visitors"}}
		if o.IncludeBots {
			rows = append(rows, []string{fmt.Sprint(b.Bots), "Bots", "bots"})
		}
		b.Tip = tipJSON(b.Title, rows...)
	}
	return t
}

// niceScale returns a round tick step and the axis top (a multiple of the step) for max.
func niceScale(maxValue int) (step, top int) {
	if maxValue <= 5 {
		return 1, max(maxValue, 1)
	}
	raw := float64(maxValue) / 4
	pow := math.Pow(10, math.Floor(math.Log10(raw)))
	switch f := raw / pow; {
	case f <= 1:
		step = int(pow)
	case f <= 2:
		step = int(2 * pow)
	case f <= 5:
		step = int(5 * pow)
	default:
		step = int(10 * pow)
	}
	top = int(math.Ceil(float64(maxValue)/float64(step))) * step
	return step, top
}

var weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}

func buildHeatmap(clicks []db_ops.Click, loc *time.Location) Heatmap {
	var counts [7][24]int
	h := Heatmap{}
	for _, c := range clicks {
		t := c.Time.In(loc)
		day := (int(t.Weekday()) + 6) % 7
		counts[day][t.Hour()]++
		h.Max = max(h.Max, counts[day][t.Hour()])
	}
	for hour := 0; hour < 24; hour++ {
		label := ""
		if hour%3 == 0 {
			label = fmt.Sprintf("%02d", hour)
		}
		h.Hours = append(h.Hours, label)
	}
	for i, wd := range weekdays {
		day := HeatDay{Name: wd.String()[:3]}
		for hour, n := range counts[i] {
			level := 0
			if n > 0 {
				level = int(math.Ceil(float64(n) * 5 / float64(h.Max)))
			}
			title := fmt.Sprintf("%s %02d:00–%02d:00", wd.String(), hour, (hour+1)%24)
			day.Cells = append(day.Cells, HeatCell{
				Count: n,
				Level: level,
				Tip:   tipJSON(title, []string{fmt.Sprint(n), "Clicks", ""}),
			})
		}
		h.Days = append(h.Days, day)
	}
	return h
}

func topRows(clicks []db_ops.Click, limit int, key func(db_ops.Click) string) []Row {
	counts := map[string]int{}
	for _, c := range clicks {
		counts[key(c)]++
	}
	rows := make([]Row, 0, len(counts))
	for label, n := range counts {
		rows = append(rows, Row{Label: label, Count: n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Label < rows[j].Label
	})
	if len(rows) > limit {
		other := Row{Label: "Other"}
		for _, r := range rows[limit-1:] {
			other.Count += r.Count
		}
		rows = append(rows[:limit-1], other)
	}
	maxCount := 0
	for _, r := range rows {
		maxCount = max(maxCount, r.Count)
	}
	for i := range rows {
		rows[i].Percent = int(math.Round(float64(rows[i].Count) * 100 / float64(len(clicks))))
		rows[i].Width = round2(float64(rows[i].Count) * 100 / float64(maxCount))
	}
	return rows
}

func CountryLabel(code string) string {
	code = strings.ToUpper(code)
	region, err := language.ParseRegion(code)
	if len(code) != 2 || code == "ZZ" || err != nil {
		return "Unknown"
	}
	name := display.English.Regions().Name(region)
	if name == "" {
		name = code
	}
	flag := string(rune(code[0])-'A'+0x1F1E6) + string(rune(code[1])-'A'+0x1F1E6)
	return flag + " " + name
}
