package internal

import (
	"sort"
	"time"
)

// AllItems is the pseudo-id meaning "any item" -- the 全部 tab in the UI.
const AllItems = ""

// weekdayIndex maps Monday..Sunday onto 0..6 (the row order used by the
// heatmap and the week cells).
func weekdayIndex(t time.Time) int { return (int(t.Weekday()) + 6) % 7 }

// dayChecked reports whether the item was FULLY completed on a day.
// Partial step progress deliberately does not count: the source app's own
// "完成次数" only counts fully completed days. id == AllItems means "at least
// one item fully completed".
func (h *History) dayChecked(date, id string) bool {
	return h.Ratio(date, id) >= 1.0
}

// dayLevel returns 0..1 completion for heatmap shading.
func (h *History) dayLevel(date, id string) float64 { return h.Ratio(date, id) }

// checkedDays returns every day on which the item was checked, ascending.
func (h *History) checkedDays(id string) []time.Time {
	out := make([]time.Time, 0, len(h.Days))
	for _, date := range h.SortedDays() {
		if !h.dayChecked(date, id) {
			continue
		}
		if t, err := parseDay(date); err == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

// parseDay reads a DayLayout string anchored at local midday, so stepping by
// whole days cannot drift across a DST boundary.
func parseDay(date string) (time.Time, error) {
	t, err := time.ParseInLocation(DayLayout, date, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.Local), nil
}

// noon normalises any time to midday of its own local date.
func noon(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, t.Location())
}

// ItemCount is how many days the item was checked.
func (h *History) ItemCount(id string) int { return len(h.checkedDays(id)) }

// ItemStreak is the current consecutive run ending today (or yesterday, when
// today has not been checked yet, so an un-started morning does not break it).
func (h *History) ItemStreak(id string, now time.Time) int {
	day := noon(now)
	if !h.dayChecked(day.Format(DayLayout), id) {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for h.dayChecked(day.Format(DayLayout), id) {
		streak++
		day = day.AddDate(0, 0, -1)
		// History is pruned to `retention` days, so a chain can never legitimately
		// exceed it; this only guards against a pathological map.
		if streak > h.retentionWindow() {
			break
		}
	}
	return streak
}

// ItemBest is the longest consecutive run on record. Bounded by the retention
// window: days older than history_days are pruned and cannot be recovered.
func (h *History) ItemBest(id string) int {
	days := h.checkedDays(id)
	if len(days) == 0 {
		return 0
	}
	best, run := 1, 1
	for i := 1; i < len(days); i++ {
		if days[i].AddDate(0, 0, -1).Format(DayLayout) == days[i-1].Format(DayLayout) {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
	}
	return best
}

// ItemRate is the share of the last `window` days (today inclusive) on which
// the item was checked.
func (h *History) ItemRate(id string, now time.Time, window int) float64 {
	if window <= 0 {
		return 0
	}
	day := noon(now)
	hit := 0
	for i := 0; i < window; i++ {
		if h.dayChecked(day.Format(DayLayout), id) {
			hit++
		}
		day = day.AddDate(0, 0, -1)
	}
	return float64(hit) / float64(window)
}

// WeeklyAverage is checks per week over the item's own span (first check to
// today), which is what the source app's "周均打卡次数"副标 reports.
func (h *History) WeeklyAverage(id string, now time.Time) float64 {
	days := h.checkedDays(id)
	if len(days) == 0 {
		return 0
	}
	first := days[0]
	span := noon(now).Sub(first).Hours() / 24
	if span < 1 {
		span = 1
	}
	weeks := span / 7
	if weeks < 1 {
		weeks = 1
	}
	return float64(len(days)) / weeks
}

// WeekCell is one day of the current week for the list view.
type WeekCell struct {
	Date    string
	Checked bool    // fully completed
	Level   float64 // 0..1 completion
	IsToday bool
	Future  bool
}

// WeekCells returns Monday..Sunday of the week containing `now`.
func (h *History) WeekCells(id string, now time.Time) []WeekCell {
	day := noon(now)
	monday := day.AddDate(0, 0, -weekdayIndex(day))
	today := day.Format(DayLayout)
	out := make([]WeekCell, 0, 7)
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i)
		key := d.Format(DayLayout)
		out = append(out, WeekCell{
			Date:    key,
			Checked: h.dayChecked(key, id),
			Level:   h.dayLevel(key, id),
			IsToday: key == today,
			Future:  key > today,
		})
	}
	return out
}

// MonthMark labels a column of the year matrix.
type MonthMark struct {
	Col   int
	Label string
}

// YearMatrix returns a Monday-first grid of completion levels (0..1) for the
// given year: grid[weekday][week]. Columns start at the week containing Jan 1,
// so the first row may be blank.
func (h *History) YearMatrix(id string, year int, now time.Time) (grid [][]float64, marks []MonthMark) {
	jan1 := time.Date(year, 1, 1, 12, 0, 0, 0, time.Local)
	dec31 := time.Date(year, 12, 31, 12, 0, 0, 0, time.Local)
	start := jan1.AddDate(0, 0, -weekdayIndex(jan1))

	cols := int(dec31.Sub(start).Hours()/24)/7 + 2
	grid = make([][]float64, 7)
	for r := range grid {
		grid[r] = make([]float64, cols)
	}

	seenMonth := map[int]bool{}
	for d := start; !d.After(dec31); d = d.AddDate(0, 0, 1) {
		if d.Year() != year {
			continue // leading days belong to the previous year
		}
		col := int(d.Sub(start).Hours()/24) / 7
		row := weekdayIndex(d)
		if col >= cols {
			break
		}
		grid[row][col] = h.dayLevel(d.Format(DayLayout), id)
		if d.Day() == 1 && !seenMonth[int(d.Month())] {
			seenMonth[int(d.Month())] = true
			marks = append(marks, MonthMark{Col: col, Label: d.Month().String()[:3]})
		}
	}
	return grid, marks
}

// WeekdayHistogram counts checks per weekday (Monday-first) for the
// "你在什么时候最能持之以恒？" chart.
func (h *History) WeekdayHistogram(id string) [7]int {
	var out [7]int
	for _, date := range h.SortedDays() {
		t, err := parseDay(date)
		if err != nil {
			continue
		}
		if h.dayChecked(date, id) {
			out[weekdayIndex(t)]++
		}
	}
	return out
}

func (h *History) retentionWindow() int {
	if h.retention > 0 {
		return h.retention
	}
	return DefaultRetentionDays
}
