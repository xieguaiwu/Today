package internal

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func statsHistory(t *testing.T) *History {
	t.Helper()
	h, err := LoadHistory(filepath.Join(t.TempDir(), "history.json"), 0)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestItemStreakAndBest(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local) // Tuesday

	// eyes: 4 consecutive days ending today
	for _, d := range []string{"2026-08-29", "2026-08-30", "2026-08-31", "2026-09-01"} {
		must(t, h.SetChecked(d, []string{"eyes"}, 12))
	}
	// a separate older run of 2, to prove Best looks at history not just now
	for _, d := range []string{"2026-08-10", "2026-08-11"} {
		must(t, h.SetChecked(d, []string{"eyes"}, 12))
	}

	if got := h.ItemStreak("eyes", now); got != 4 {
		t.Errorf("current streak = %d, want 4", got)
	}
	if got := h.ItemBest("eyes"); got != 4 {
		t.Errorf("best streak = %d, want 4", got)
	}
	if got := h.ItemCount("eyes"); got != 6 {
		t.Errorf("count = %d, want 6", got)
	}
}

func TestItemStreakToleratesUnstartedToday(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.Local)
	must(t, h.SetChecked("2026-08-30", []string{"eyes"}, 12))
	must(t, h.SetChecked("2026-08-31", []string{"eyes"}, 12))
	if got := h.ItemStreak("eyes", now); got != 2 {
		t.Errorf("streak = %d, want 2 (today not checked yet)", got)
	}
}

func TestStreakIsPerItem(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	must(t, h.SetChecked("2026-09-01", []string{"eyes"}, 12))
	must(t, h.SetChecked("2026-08-31", []string{"nose"}, 12))

	if got := h.ItemStreak("eyes", now); got != 1 {
		t.Errorf("eyes streak = %d, want 1", got)
	}
	// nose was checked yesterday only: the chain legitimately ends on yesterday,
	// because an un-started today must not break a live streak.
	if got := h.ItemStreak("nose", now); got != 1 {
		t.Errorf("nose streak = %d, want 1 (ends yesterday)", got)
	}
	// ...but it does not extend to today.
	if got := h.ItemStreak("nose", now.AddDate(0, 0, 2)); got != 0 {
		t.Errorf("nose streak two days later = %d, want 0", got)
	}
	if got := h.ItemCount("nose"); got != 1 {
		t.Errorf("nose count = %d, want 1", got)
	}
}

func TestAllItemsCountsAnyCheck(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	must(t, h.SetChecked("2026-08-31", []string{"eyes"}, 12))
	must(t, h.SetChecked("2026-09-01", []string{"brain"}, 12))

	if got := h.ItemStreak(AllItems, now); got != 2 {
		t.Errorf("全部 streak = %d, want 2 (different items still count the day)", got)
	}
	if got := h.ItemCount(AllItems); got != 2 {
		t.Errorf("全部 count = %d, want 2", got)
	}
}

func TestAllItemsCountIsSumOfChecks(t *testing.T) {
	// The source app's 全部 card sums per-habit completions (24+0+7+2+8 = 41),
	// so two habits on one day must count twice.
	h := statsHistory(t)
	must(t, h.SetChecked("2026-09-01", []string{"eyes", "nose"}, 12))

	total := 0
	for _, id := range []string{"eyes", "nose"} {
		total += h.ItemCount(id)
	}
	if total != 2 {
		t.Fatalf("setup: expected 2 per-item completions, got %d", total)
	}
	if days := h.ItemCount(AllItems); days != 1 {
		t.Errorf("全部-as-days = %d, want 1 (distinct days differ from completions)", days)
	}
}

func TestItemRate90(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	for _, d := range []string{"2026-08-30", "2026-08-31", "2026-09-01"} {
		must(t, h.SetChecked(d, []string{"eyes"}, 12))
	}
	// outside the 90-day window
	must(t, h.SetChecked("2026-01-01", []string{"eyes"}, 12))

	got := h.ItemRate("eyes", now, 90)
	if want := 3.0 / 90.0; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("rate90 = %v, want %v", got, want)
	}
}

func TestWeeklyAverage(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	// 24 checks over 24 days = 3.43 weeks -> ~7.0/week (matches the app's anki card)
	for i := 0; i < 24; i++ {
		d := now.AddDate(0, 0, -(23 - i)).Format(DayLayout)
		must(t, h.SetChecked(d, []string{"anki"}, 12))
	}
	got := h.WeeklyAverage("anki", now)
	if got < 6.5 || got > 7.5 {
		t.Errorf("weekly average = %.2f, want ~7.0", got)
	}
	if got := h.WeeklyAverage("absent", now); got != 0 {
		t.Errorf("weekly average for unknown item = %v, want 0", got)
	}
}

func TestWeekCellsMondayFirst(t *testing.T) {
	h := statsHistory(t)
	tue := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)     // Tuesday
	must(t, h.SetChecked("2026-08-31", []string{"eyes"}, 12)) // Monday
	must(t, h.SetChecked("2026-09-01", []string{"eyes"}, 12)) // Tuesday

	cells := h.WeekCells("eyes", tue)
	if len(cells) != 7 {
		t.Fatalf("got %d cells, want 7", len(cells))
	}
	if cells[0].Date != "2026-08-31" {
		t.Errorf("week starts at %s, want 2026-08-31 (Monday)", cells[0].Date)
	}
	if cells[6].Date != "2026-09-06" {
		t.Errorf("week ends at %s, want 2026-09-06 (Sunday)", cells[6].Date)
	}
	if !cells[0].Checked || !cells[1].Checked {
		t.Error("Mon/Tue should be checked")
	}
	if !cells[1].IsToday {
		t.Error("Tue should be flagged today")
	}
	for i := 2; i < 7; i++ {
		if !cells[i].Future {
			t.Errorf("cell %d (%s) should be future", i, cells[i].Date)
		}
	}
}

func TestWeekCellsOnSundayStillBelongsToThatWeek(t *testing.T) {
	h := statsHistory(t)
	sun := time.Date(2026, 9, 6, 12, 0, 0, 0, time.Local) // Sunday
	cells := h.WeekCells("eyes", sun)
	if cells[0].Date != "2026-08-31" || cells[6].Date != "2026-09-06" {
		t.Errorf("Sunday week = %s..%s, want 2026-08-31..2026-09-06", cells[0].Date, cells[6].Date)
	}
	if !cells[6].IsToday {
		t.Error("Sunday should be today")
	}
}

func TestYearMatrixShapeAndPlacement(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	must(t, h.SetChecked("2026-09-01", []string{"eyes"}, 12)) // Tuesday
	must(t, h.SetChecked("2026-01-01", []string{"eyes"}, 12)) // Thursday

	grid, marks := h.YearMatrix("eyes", 2026, now)
	if len(grid) != 7 {
		t.Fatalf("got %d rows, want 7", len(grid))
	}
	find := func(date string) (int, int, bool) {
		d, err := parseDay(date)
		if err != nil {
			return 0, 0, false
		}
		jan1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
		start := jan1.AddDate(0, 0, -weekdayIndex(jan1))
		col := int(d.Sub(start).Hours()/24) / 7
		return col, weekdayIndex(d), true
	}
	c, r, _ := find("2026-09-01")
	if grid[r][c] < 1 {
		t.Errorf("2026-09-01 should be fully lit, got %v", grid[r][c])
	}
	c2, r2, _ := find("2026-01-01")
	if grid[r2][c2] < 1 {
		t.Error("2026-01-01 should be lit")
	}
	if len(marks) == 0 {
		t.Error("expected month labels")
	}
	if marks[0].Col != 0 {
		t.Errorf("first month mark at col %d, want 0", marks[0].Col)
	}
}

func TestYearMatrixExcludesOtherYears(t *testing.T) {
	h := statsHistory(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.Local)
	must(t, h.SetChecked("2025-12-31", []string{"eyes"}, 12))
	must(t, h.SetChecked("2027-01-01", []string{"eyes"}, 12))

	grid, _ := h.YearMatrix("eyes", 2026, now)
	lit := 0
	for r := 0; r < 7; r++ {
		for c := 0; c < len(grid[r]); c++ {
			if grid[r][c] > 0 {
				lit++
			}
		}
	}
	if lit != 0 {
		t.Errorf("neighbouring-year checks leaked into 2026: %d lit", lit)
	}
}

func TestWeekdayHistogram(t *testing.T) {
	h := statsHistory(t)
	// 2026-08-31 Mon, 2026-09-01 Tue, 2026-09-07 Mon
	for _, d := range []string{"2026-08-31", "2026-09-01", "2026-09-07"} {
		must(t, h.SetChecked(d, []string{"eyes"}, 12))
	}
	got := h.WeekdayHistogram("eyes")
	if got[0] != 2 {
		t.Errorf("Monday count = %d, want 2", got[0])
	}
	if got[1] != 1 {
		t.Errorf("Tuesday count = %d, want 1", got[1])
	}
	total := 0
	for _, n := range got {
		total += n
	}
	if total != h.ItemCount("eyes") {
		t.Errorf("histogram total %d != item count %d", total, h.ItemCount("eyes"))
	}
}

func TestRGBParsing(t *testing.T) {
	cases := []struct {
		in      string
		r, g, b int
		ok      bool
	}{
		{"#3B82F6", 0x3B, 0x82, 0xF6, true},
		{"#fff", 0xFF, 0xFF, 0xFF, true},
		{"F87171", 0xF8, 0x71, 0x71, true},
		{"#12", 0, 0, 0, false},
		{"#GGGGGG", 0, 0, 0, false},
		{"", 0, 0, 0, false},
	}
	for _, c := range cases {
		r, g, b, ok := rgb(c.in)
		if ok != c.ok || (ok && (r != c.r || g != c.g || b != c.b)) {
			t.Errorf("rgb(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)", c.in, r, g, b, ok, c.r, c.g, c.b, c.ok)
		}
	}
}

func TestValidateColor(t *testing.T) {
	for _, good := range []string{"#fff", "#FFF", "#3B82F6"} {
		if err := validateColor(good); err != nil {
			t.Errorf("%q should be valid: %v", good, err)
		}
	}
	for _, bad := range []string{"red", "#12", "#12345", "3B82F6G", "#GGHHII"} {
		if err := validateColor(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestColorOrDefaultFallsBackToPalette(t *testing.T) {
	items := normalizeItems([]Item{{Label: "A"}, {Label: "B"}, {Label: "C", Color: "#123456"}})
	if got := items[0].ColorOrDefault(0); got != Palette[0] {
		t.Errorf("item0 color = %q, want palette %q", got, Palette[0])
	}
	if got := items[2].ColorOrDefault(2); got != "#123456" {
		t.Errorf("explicit color lost: %q", got)
	}
	// wraps past the end of the palette
	if got := items[1].ColorOrDefault(len(Palette)); got != Palette[0] {
		t.Errorf("palette wrap = %q, want %q", got, Palette[0])
	}
}

func TestDisplayWidthHandlesCJK(t *testing.T) {
	if got := displayWidth("无氧锻炼"); got != 8 {
		t.Errorf("CJK width = %d, want 8", got)
	}
	if got := displayWidth("abc"); got != 3 {
		t.Errorf("ascii width = %d, want 3", got)
	}
	if got := displayWidth("\x1b[38;2;1;2;3m■\x1b[0m"); got != 1 {
		t.Errorf("ANSI should not count: %d", got)
	}
}

func TestWeekCellsGlyphsAreNotColourOnly(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	cells := []WeekCell{
		{Date: "a", Checked: true, Level: 1},
		{Date: "b", Checked: true, Level: 1, IsToday: true},
		{Date: "c", IsToday: true},
		{Date: "d"},
		{Date: "e", Future: true},
		{Date: "f", Level: 0.5},
		{Date: "g", Level: 0.8},
		{Date: "h", Level: 0.2},
	}
	got := weekCells(cells, "#3B82F6")
	if strings.Contains(got, "\x1b[") {
		t.Errorf("NO_COLOR leaked ANSI: %q", got)
	}
	// Every state must be distinguishable by glyph alone, since the shading is
	// the only place step progress is shown.
	for _, want := range []string{cellFull, cellToday, cellTodayO, cellOff, cellDim, cellMid, cellHigh, cellLow} {
		if !strings.Contains(got, want) {
			t.Errorf("glyph %q missing from %q", want, got)
		}
	}
	if len([]rune(got)) != len(cells) {
		t.Errorf("expected one cell per day, got %d runes for %d cells", len([]rune(got)), len(cells))
	}
}
