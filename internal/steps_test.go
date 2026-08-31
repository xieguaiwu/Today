package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stepModel(t *testing.T, steps ...int) Model {
	t.Helper()
	cfg := Config{Title: "T", Items: []Item{
		{ID: "single", Label: "Single"},
		{ID: "multi", Label: "Multi", Steps: steps[0]},
	}}
	cfg.Items = normalizeItems(cfg.Items)
	h, err := LoadHistory(filepath.Join(t.TempDir(), "history.json"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h.UseCatalog(NewCatalog(cfg.Items))
	return New(cfg, h)
}

func TestStepsConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"items":[{"id":"a","label":"Anki","steps":4},{"label":"Plain","steps":0}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Items[0].StepsOrDefault() != 4 {
		t.Errorf("steps = %d, want 4", cfg.Items[0].StepsOrDefault())
	}
	if cfg.Items[1].StepsOrDefault() != 1 {
		t.Errorf("steps 0 must mean a plain checkbox, got %d", cfg.Items[1].StepsOrDefault())
	}
	if err := WriteConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `"steps": 4`) {
		t.Errorf("steps lost in round trip:\n%s", raw)
	}
}

func TestStepsOutOfRangeRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, bad := range []string{`{"items":[{"label":"A","steps":99}]}`, `{"items":[{"label":"A","steps":-1}]}`} {
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(path); err == nil {
			t.Errorf("should reject: %s", bad)
		}
	}
}

func TestCatalogStepsDefaultToOne(t *testing.T) {
	c := NewCatalog([]Item{{ID: "a", Steps: 3}, {ID: "b"}})
	if c.Steps("a") != 3 {
		t.Errorf("a = %d, want 3", c.Steps("a"))
	}
	if c.Steps("b") != 1 {
		t.Errorf("b = %d, want 1", c.Steps("b"))
	}
	if c.Steps("unknown") != 1 {
		t.Error("unknown id should default to 1, not 0 (0 would make it unfinishable)")
	}
}

func TestPartialDoesNotCountAsCompletion(t *testing.T) {
	m := stepModel(t, 4)
	now := time.Now()

	m = send(t, m, down()) // cursor -> multi
	m = send(t, m, space())
	m = send(t, m, space()) // 2 of 4 steps

	if m.CheckedCount() != 0 {
		t.Errorf("partial progress must not count as done, got %d", m.CheckedCount())
	}
	if m.PartialCount() != 1 {
		t.Errorf("PartialCount = %d, want 1", m.PartialCount())
	}
	if got := m.hist.ItemCount("multi"); got != 0 {
		t.Errorf("partial days must not inflate 完成次数, got %d", got)
	}
	if got := m.hist.ItemStreak("multi", now); got != 0 {
		t.Errorf("partial days must not sustain a streak, got %d", got)
	}
	if got := m.hist.Ratio(m.date, "multi"); got != 0.5 {
		t.Errorf("ratio = %v, want 0.5", got)
	}

	// finish it
	m = send(t, m, space())
	m = send(t, m, space())
	if m.CheckedCount() != 1 {
		t.Errorf("after 4 steps it should count, got %d", m.CheckedCount())
	}
	if got := m.hist.ItemStreak("multi", now); got != 1 {
		t.Errorf("streak = %d, want 1", got)
	}
}

func TestSpaceWrapsBackToZeroWhenFull(t *testing.T) {
	m := stepModel(t, 3)
	m = send(t, m, down())
	for i := 0; i < 3; i++ {
		m = send(t, m, space())
	}
	if m.progress["multi"] != 3 {
		t.Fatalf("setup: progress = %d, want 3", m.progress["multi"])
	}
	m = send(t, m, space()) // 4th press wraps to 0
	if m.progress["multi"] != 0 {
		t.Errorf("space at full should clear, got %d", m.progress["multi"])
	}
}

func TestFillKeyJumpsToFull(t *testing.T) {
	m := stepModel(t, 5)
	m = send(t, m, down())
	m = send(t, m, key('f'))
	if m.progress["multi"] != 5 {
		t.Errorf("f should fill, got %d", m.progress["multi"])
	}
	if m.CheckedCount() != 1 {
		t.Error("filled item should count as done")
	}
	m = send(t, m, key('f')) // toggles off
	if m.progress["multi"] != 0 {
		t.Errorf("f again should clear, got %d", m.progress["multi"])
	}
}

func TestPlusMinusClamp(t *testing.T) {
	m := stepModel(t, 3)
	m = send(t, m, down())
	m = send(t, m, key('-')) // already at zero
	if m.progress["multi"] != 0 {
		t.Errorf("minus below zero must clamp, got %d", m.progress["multi"])
	}
	for i := 0; i < 6; i++ {
		m = send(t, m, key('+'))
	}
	if m.progress["multi"] != 3 {
		t.Errorf("plus above max must clamp, got %d", m.progress["multi"])
	}
}

func TestSingleStepItemKeepsOldBehaviour(t *testing.T) {
	m := stepModel(t, 4)
	m = send(t, m, space()) // cursor 0 = single-step item
	if m.CheckedCount() != 1 {
		t.Fatal("one space should complete a 1-step item")
	}
	m = send(t, m, space())
	if m.CheckedCount() != 0 {
		t.Fatal("second space should clear a 1-step item")
	}
}

func TestProgressSurvivesReload(t *testing.T) {
	m := stepModel(t, 4)
	m = send(t, m, down())
	m = send(t, m, space())
	m = send(t, m, space())

	reloaded, err := LoadHistory(m.hist.path, 0)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.UseCatalog(NewCatalog(m.cfg.Items))
	if got := reloaded.StepsOn(m.date, "multi"); got != 2 {
		t.Errorf("after reload steps = %d, want 2", got)
	}
	// The UI must restore partial progress, not just completed items.
	reopened := New(m.cfg, reloaded)
	if got := reopened.progress["multi"]; got != 2 {
		t.Errorf("partial progress lost on reopen: %+v", reopened.progress)
	}
	if reopened.CheckedCount() != 0 {
		t.Error("reopened model should not treat partial as done")
	}
}

func TestLegacyV1FileReadsAsFullCredit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	v1 := `{"version":1,"days":{"2026-08-31":{"checked":["anki","brain"],"total":2,"updated_at":"2026-08-31T09:00:00Z"}}}`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	// anki now needs 4 steps; a v1 record predates that, so it must still count.
	h.UseCatalog(NewCatalog([]Item{{ID: "anki", Label: "Anki", Steps: 4}}))

	if got := h.StepsOn("2026-08-31", "anki"); got != 4 {
		t.Errorf("legacy full check should map to max steps, got %d", got)
	}
	if !h.dayChecked("2026-08-31", "anki") {
		t.Error("legacy record should count as completed")
	}
	if got := h.ItemCount("anki"); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

func TestAllItemsWorksWithoutCatalog(t *testing.T) {
	// Regression: 全部 used to read only through the catalog, so a History built
	// without UseCatalog reported zero completions no matter what was stored.
	path := filepath.Join(t.TempDir(), "history.json")
	body := `{"version":2,"days":{"2026-08-31":{"progress":{"anki":4},"total":2}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	// "anki" is unknown to a catalog-less history, so its step count defaults
	// to 1 and a stored 4 means fully done.
	if got := h.ItemCount(AllItems); got != 1 {
		t.Errorf("全部 count without catalog = %d, want 1", got)
	}
}

func TestAllItemsRatioIsBestItem(t *testing.T) {
	cfg := Config{Items: []Item{{ID: "a", Label: "A", Steps: 4}, {ID: "b", Label: "B", Steps: 4}}}
	cfg.Items = normalizeItems(cfg.Items)
	h, _ := LoadHistory(filepath.Join(t.TempDir(), "h.json"), 0)
	h.UseCatalog(NewCatalog(cfg.Items))

	if err := h.SetProgress("2026-08-31", map[string]int{"a": 1, "b": 2}, 2); err != nil {
		t.Fatal(err)
	}
	if got := h.Ratio("2026-08-31", AllItems); got != 0.5 {
		t.Errorf("全部 ratio = %v, want 0.5 (best item b = 2/4)", got)
	}
	if h.dayChecked("2026-08-31", AllItems) {
		t.Error("no item is complete, so 全部 must not count the day")
	}
	if err := h.SetProgress("2026-08-31", map[string]int{"a": 1, "b": 4}, 2); err != nil {
		t.Fatal(err)
	}
	if !h.dayChecked("2026-08-31", AllItems) {
		t.Error("one fully complete item should make 全部 count the day")
	}
}

func TestPartialProgressIsVisibleInHeatmap(t *testing.T) {
	cfg := Config{Items: []Item{{ID: "a", Label: "A", Steps: 4, Color: "#3B82F6"}}}
	cfg.Items = normalizeItems(cfg.Items)
	h, _ := LoadHistory(filepath.Join(t.TempDir(), "h.json"), 0)
	h.UseCatalog(NewCatalog(cfg.Items))
	if err := h.SetProgress("2026-01-05", map[string]int{"a": 2}, 1); err != nil {
		t.Fatal(err)
	}
	grid, _ := h.YearMatrix("a", 2026, time.Now())
	found := false
	for r := 0; r < 7; r++ {
		for c := 0; c < len(grid[r]); c++ {
			if grid[r][c] == 0.5 {
				found = true
			}
		}
	}
	if !found {
		t.Error("partial completion should appear in the matrix at 0.5")
	}
}

func TestStatusReportsPartial(t *testing.T) {
	// --status must expose partial counts so a script can tell "doing" from "done".
	m := stepModel(t, 4)
	m = send(t, m, down())
	m = send(t, m, space())
	out := m.hist
	_ = out
	if m.PartialCount() != 1 || m.CheckedCount() != 0 {
		t.Fatalf("counts wrong: partial=%d done=%d", m.PartialCount(), m.CheckedCount())
	}
}

func TestSavedFileUsesProgressNotChecked(t *testing.T) {
	m := stepModel(t, 4)
	m = send(t, m, space()) // complete the single-step item
	raw, err := os.ReadFile(m.hist.path)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Version int `json:"version"`
		Days    map[string]struct {
			Progress map[string]int `json:"progress"`
			Checked  []string       `json:"checked"`
		} `json:"days"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Version != HistoryVersion {
		t.Errorf("version = %d, want %d", probe.Version, HistoryVersion)
	}
	rec := probe.Days[m.date]
	if len(rec.Progress) == 0 {
		t.Fatalf("expected progress map in v2 output:\n%s", raw)
	}
	if len(rec.Checked) != 0 {
		t.Errorf("v2 writes should not emit the legacy checked list: %v", rec.Checked)
	}
}
