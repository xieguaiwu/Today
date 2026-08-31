package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestHistory(t *testing.T, retention int) *History {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.json")
	h, err := LoadHistory(path, retention)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	return h
}

func TestLoadHistoryMissingFileIsEmpty(t *testing.T) {
	h := newTestHistory(t, 0)
	if len(h.Days) != 0 {
		t.Fatalf("fresh history should be empty, got %d days", len(h.Days))
	}
	if h.Notice != "" {
		t.Errorf("fresh history should have no notice, got %q", h.Notice)
	}
}

func TestSetCheckedSurvivesReload(t *testing.T) {
	h := newTestHistory(t, 0)
	if err := h.SetChecked("2026-08-31", []string{"eyes", "nose"}, 7); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadHistory(h.path, 0)
	if err != nil {
		t.Fatal(err)
	}
	set := reloaded.CheckedSet("2026-08-31")
	if len(set) != 2 {
		t.Fatalf("lost state on reload: %+v", reloaded.Days)
	}
	if _, ok := set["eyes"]; !ok {
		t.Error("eyes not persisted")
	}
	if rec := reloaded.Day("2026-08-31"); rec.Total != 7 {
		t.Errorf("total = %d, want 7", rec.Total)
	}
}

func TestUncheckingEverythingRemovesTheDay(t *testing.T) {
	h := newTestHistory(t, 0)
	if err := h.SetChecked("2026-08-31", []string{"eyes"}, 7); err != nil {
		t.Fatal(err)
	}
	if err := h.SetChecked("2026-08-31", nil, 7); err != nil {
		t.Fatal(err)
	}
	if len(h.Days) != 0 {
		t.Fatalf("empty day should be pruned, got %+v", h.Days)
	}
	if h.CheckedCount() != 0 {
		t.Error("CheckedCount should ignore empty days")
	}
}

func TestResetDay(t *testing.T) {
	h := newTestHistory(t, 0)
	_ = h.SetChecked("2026-08-31", []string{"eyes"}, 7)
	_ = h.SetChecked("2026-08-30", []string{"eyes"}, 7)
	if err := h.ResetDay("2026-08-31"); err != nil {
		t.Fatal(err)
	}
	if len(h.Days) != 1 || h.Day("2026-08-30") == nil {
		t.Fatalf("reset touched the wrong day: %+v", h.Days)
	}
	// Resetting an already-empty day is a no-op, not an error.
	if err := h.ResetDay("2020-01-01"); err != nil {
		t.Errorf("reset of absent day: %v", err)
	}
}

func TestAtomicWriteLeavesNoTempFiles(t *testing.T) {
	h := newTestHistory(t, 0)
	if err := h.SetChecked("2026-08-31", []string{"eyes"}, 7); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(h.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "history.json" {
			t.Errorf("stray file left behind: %s", e.Name())
		}
	}
}

func TestHistoryFileIsOwnerOnly(t *testing.T) {
	h := newTestHistory(t, 0)
	if err := h.SetChecked("2026-08-31", []string{"eyes"}, 7); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(h.path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("history perms = %o, want 600 (personal health data)", perm)
	}
}

func TestCorruptHistoryIsQuarantinedNotFatal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatalf("corrupt history must not abort the tool: %v", err)
	}
	if h.Notice == "" {
		t.Error("expected a user-visible notice")
	}
	if len(h.Days) != 0 {
		t.Errorf("expected fresh state, got %+v", h.Days)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	var quarantined int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "history.json.corrupt-") {
			quarantined++
		}
	}
	if quarantined != 1 {
		t.Errorf("bad file should be preserved once as .corrupt-*, found %d", quarantined)
	}
}

func TestNilDayRecordsDroppedOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"days":{"2026-01-01":null}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Days) != 0 {
		t.Fatalf("null record survived: %+v", h.Days)
	}
}

func TestPruneHonoursRetentionWindow(t *testing.T) {
	h := newTestHistory(t, 30)
	now := time.Now()
	old := now.AddDate(0, 0, -90).Format(DayLayout)
	recent := now.AddDate(0, 0, -3).Format(DayLayout)
	_ = h.SetChecked(old, []string{"eyes"}, 7)
	_ = h.SetChecked(recent, []string{"eyes"}, 7)
	if h.Day(old) != nil {
		t.Fatalf("day outside the 30-day window was kept: %s", old)
	}
	if h.Day(recent) == nil {
		t.Errorf("day inside the window was wrongly pruned: %s", recent)
	}
}

func TestStreakCountsBackFromToday(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.Local)
	h := newTestHistory(t, 0)
	for _, d := range []string{"2026-08-29", "2026-08-30", "2026-08-31"} {
		if err := h.SetChecked(d, []string{"eyes"}, 7); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.Streak(now); got != 3 {
		t.Errorf("streak = %d, want 3", got)
	}
}

func TestStreakAllowsUnstartedToday(t *testing.T) {
	now := time.Date(2026, 8, 31, 8, 0, 0, 0, time.Local)
	h := newTestHistory(t, 0)
	_ = h.SetChecked("2026-08-29", []string{"eyes"}, 7)
	_ = h.SetChecked("2026-08-30", []string{"eyes"}, 7)
	if got := h.Streak(now); got != 2 {
		t.Errorf("streak = %d, want 2 (today not started yet)", got)
	}
}

func TestStreakBreaksOnGap(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.Local)
	h := newTestHistory(t, 0)
	_ = h.SetChecked("2026-08-31", []string{"eyes"}, 7)
	_ = h.SetChecked("2026-08-29", []string{"eyes"}, 7) // 08-30 missing
	if got := h.Streak(now); got != 1 {
		t.Errorf("streak = %d, want 1", got)
	}
}

func TestStreakZeroWhenEmpty(t *testing.T) {
	h := newTestHistory(t, 0)
	if got := h.Streak(time.Now()); got != 0 {
		t.Errorf("streak = %d, want 0", got)
	}
}

func TestCompleteCountUsesPerDayTotal(t *testing.T) {
	h := newTestHistory(t, 0)
	_ = h.SetChecked("2026-08-30", []string{"a", "b"}, 2) // complete
	_ = h.SetChecked("2026-08-31", []string{"a"}, 3)      // partial
	if got := h.CompleteCount(); got != 1 {
		t.Errorf("complete days = %d, want 1", got)
	}
	if got := h.CheckedCount(); got != 2 {
		t.Errorf("checked days = %d, want 2", got)
	}
}

func TestBadTimestampDoesNotDestroyChecklistData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	body := `{
	  "version": 1,
	  "days": {
	    "2026-08-31": {"checked": ["eyes"], "total": 7, "updated_at": "not-a-time"},
	    "2026-08-30": {"checked": ["nose"], "total": 7, "updated_at": "2026-08-30T09:00:00+08:00"},
	    "2026-08-29": {"checked": ["skin"], "total": 7}
	  }
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatalf("a cosmetic bad timestamp must not fail the load: %v", err)
	}
	if h.Notice != "" {
		t.Errorf("should not have quarantined anything, notice = %q", h.Notice)
	}
	if len(h.Days) != 3 {
		t.Fatalf("lost days: %+v", h.Days)
	}
	if got := h.SortedDays(); strings.Join(got, ",") != "2026-08-29,2026-08-30,2026-08-31" {
		t.Errorf("days = %v", got)
	}
	if !h.Day("2026-08-31").Updated.IsZero() {
		t.Error("bad timestamp should decode to zero time")
	}
	if h.Day("2026-08-30").Updated.Year() != 2026 {
		t.Error("valid timestamp should still parse")
	}
}

func TestSortedDays(t *testing.T) {
	h := newTestHistory(t, 0)
	_ = h.SetChecked("2026-08-31", []string{"a"}, 1)
	_ = h.SetChecked("2026-08-29", []string{"a"}, 1)
	_ = h.SetChecked("2026-08-30", []string{"a"}, 1)
	got := h.SortedDays()
	want := []string{"2026-08-29", "2026-08-30", "2026-08-31"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SortedDays = %v, want %v", got, want)
	}
}

func TestDataPathPrecedence(t *testing.T) {
	t.Setenv("TODAY_DATA", "/env/history.json")
	t.Setenv("XDG_DATA_HOME", "/xdg")
	if got, _ := DataPath("/flag.json"); got != "/flag.json" {
		t.Errorf("flag should win, got %q", got)
	}
	if got, _ := DataPath(""); got != "/env/history.json" {
		t.Errorf("env should beat XDG, got %q", got)
	}
	t.Setenv("TODAY_DATA", "")
	if got, _ := DataPath(""); filepath.ToSlash(got) != "/xdg/Today/history.json" {
		t.Errorf("XDG not honoured, got %q", got)
	}
}
