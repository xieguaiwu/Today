package internal

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testModel(t *testing.T) Model {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Items[0].Hint = "远眺 20 分钟"
	path := filepath.Join(t.TempDir(), "history.json")
	hist, err := LoadHistory(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, hist)
}

func send(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return mm
}

func key(r rune) tea.KeyMsg          { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
func space() tea.KeyMsg              { return tea.KeyMsg{Type: tea.KeySpace} }
func up() tea.KeyMsg                 { return tea.KeyMsg{Type: tea.KeyUp} }
func down() tea.KeyMsg               { return tea.KeyMsg{Type: tea.KeyDown} }
func arrow(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func TestNewBindsToday(t *testing.T) {
	m := testModel(t)
	if m.date != time.Now().Format(DayLayout) {
		t.Errorf("date = %q, want today", m.date)
	}
	if len(m.checked) != 0 {
		t.Errorf("fresh model should have nothing checked, got %v", m.checked)
	}
	if m.Init() == nil {
		t.Error("Init should return the rollover ticker command")
	}
}

func TestTogglePersistsImmediately(t *testing.T) {
	m := testModel(t)
	m = send(t, m, space())
	if m.CheckedCount() != 1 {
		t.Fatalf("CheckedCount = %d, want 1", m.CheckedCount())
	}
	// The point of v0.2.0: state must already be on disk, not waiting for quit.
	reloaded, err := LoadHistory(m.hist.path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.CheckedSet(m.date)) != 1 {
		t.Errorf("toggle was not persisted: %+v", reloaded.Days)
	}

	m = send(t, m, space())
	if m.CheckedCount() != 0 {
		t.Errorf("second toggle should uncheck, got %d", m.CheckedCount())
	}
	reloaded, _ = LoadHistory(m.hist.path, 0)
	if len(reloaded.Days) != 0 {
		t.Errorf("unchecking all should drop the day record: %+v", reloaded.Days)
	}
}

func TestEnterAlsoToggles(t *testing.T) {
	m := send(t, testModel(t), arrow(tea.KeyEnter))
	if m.CheckedCount() != 1 {
		t.Errorf("enter should toggle, got %d", m.CheckedCount())
	}
}

func TestCursorStaysInBounds(t *testing.T) {
	m := testModel(t)
	for i := 0; i < 5; i++ {
		m = send(t, m, up())
	}
	if m.cursor != 0 {
		t.Errorf("cursor escaped top: %d", m.cursor)
	}
	last := len(m.cfg.Items) - 1
	for i := 0; i < last+5; i++ {
		m = send(t, m, down())
	}
	if m.cursor != last {
		t.Errorf("cursor escaped bottom: %d, want %d", m.cursor, last)
	}
}

func TestVimKeysMoveCursor(t *testing.T) {
	m := testModel(t)
	m = send(t, m, key('j'))
	if m.cursor != 1 {
		t.Errorf("j should move down, cursor=%d", m.cursor)
	}
	m = send(t, m, key('k'))
	if m.cursor != 0 {
		t.Errorf("k should move up, cursor=%d", m.cursor)
	}
}

func TestQuitKeysReturnQuitCommand(t *testing.T) {
	for _, k := range []tea.KeyMsg{key('q'), {Type: tea.KeyCtrlC}} {
		_, cmd := testModel(t).Update(k)
		if cmd == nil {
			t.Errorf("key %q should return tea.Quit", k.String())
		}
	}
}

func TestResetDayClearsAndPersists(t *testing.T) {
	m := send(t, testModel(t), space())
	if m.CheckedCount() != 1 {
		t.Fatal("setup failed")
	}
	m = send(t, m, key('r'))
	if m.CheckedCount() != 0 {
		t.Errorf("r should clear today, got %d", m.CheckedCount())
	}
	reloaded, _ := LoadHistory(m.hist.path, 0)
	if len(reloaded.Days) != 0 {
		t.Errorf("reset not persisted: %+v", reloaded.Days)
	}
}

func TestCoalescedKeystrokeBurstIsNotDropped(t *testing.T) {
	// Regression lock: "jj" delivered in a single read used to match no binding,
	// so fast typers silently lost keystrokes.
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jj")})
	if m.cursor != 2 {
		t.Fatalf("burst 'jj' should move twice, cursor=%d", m.cursor)
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("  ")})
	if m.CheckedCount() != 0 {
		t.Fatalf("two coalesced spaces on one item = check then uncheck, want 0, got %d", m.CheckedCount())
	}
	// A single space in a burst must still register.
	m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m.CheckedCount() != 1 {
		t.Fatalf("single space should check, got %d", m.CheckedCount())
	}
	m2 := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j ")})
	if m2.cursor != 1 || m2.CheckedCount() != 1 {
		t.Errorf("burst 'j ' should move then toggle: cursor=%d checked=%d", m2.cursor, m2.CheckedCount())
	}
	reloaded, _ := LoadHistory(m2.hist.path, 0)
	if len(reloaded.CheckedSet(m2.date)) != 1 {
		t.Errorf("burst toggle not persisted: %+v", reloaded.Days)
	}
}

func TestBurstStopsAtQuit(t *testing.T) {
	m := testModel(t)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jq")})
	if cmd == nil {
		t.Error("burst containing q must request quit")
	}
	if mm := next.(Model); mm.cursor != 1 {
		t.Errorf("keystrokes before q should still apply, cursor=%d", mm.cursor)
	}
}

func TestViewHasNoTutorialLeftovers(t *testing.T) {
	m := testModel(t)
	v := m.View()
	// v0.1.0 shipped the bubbletea tutorial header verbatim.
	for _, bad := range []string{"What to buy?", "Press q to quit."} {
		if strings.Contains(v, bad) {
			t.Errorf("View still contains tutorial leftover %q", bad)
		}
	}
	if !strings.Contains(strings.ReplaceAll(v, "\x1b[2m", ""), DefaultTitle) {
		t.Errorf("View missing the configured title:\n%s", v)
	}
	if !strings.Contains(v, m.date) {
		t.Errorf("View missing the date:\n%s", v)
	}
	if !strings.Contains(v, "0/7") {
		t.Errorf("View missing progress counter:\n%s", v)
	}
	for _, it := range m.cfg.Items {
		if !strings.Contains(v, it.Label) {
			t.Errorf("View missing item %q", it.Label)
		}
	}
}

func TestViewShowsHintOnlyOnCursor(t *testing.T) {
	m := testModel(t) // hint lives on item 0
	if !strings.Contains(m.View(), "远眺 20 分钟") {
		t.Error("hint should show on the cursor line")
	}
	m = send(t, m, down())
	if strings.Contains(m.View(), "远眺 20 分钟") {
		t.Error("hint of an off-cursor item should be hidden")
	}
}

func TestViewMarksCheckedItems(t *testing.T) {
	m := send(t, testModel(t), space())
	if !strings.Contains(m.View(), "[x]") {
		t.Error("checked item should render [x]")
	}
	if !strings.Contains(m.View(), "[ ]") {
		t.Error("unchecked items should still render [ ]")
	}
}

func TestNoColorEnvStripsAnsi(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if strings.Contains(testModel(t).View(), "\x1b[2m") {
		t.Error("NO_COLOR should disable ANSI sequences")
	}
}

func TestRolloverAdoptsNewDay(t *testing.T) {
	m := testModel(t)
	m = send(t, m, space())
	if m.CheckedCount() != 1 {
		t.Fatal("setup failed")
	}
	tomorrow := time.Now().AddDate(0, 0, 1)
	m.rollDate(tomorrow)

	if m.date != tomorrow.Format(DayLayout) {
		t.Errorf("date = %q, want %q", m.date, tomorrow.Format(DayLayout))
	}
	if m.CheckedCount() != 0 {
		t.Errorf("new day must start empty, got %d", m.CheckedCount())
	}
	if m.cursor != 0 {
		t.Errorf("cursor should reset on rollover, got %d", m.cursor)
	}
}

func TestRolloverRestoresExistingRecord(t *testing.T) {
	m := testModel(t)
	prev := time.Now().AddDate(0, 0, -1)
	if err := m.hist.SetChecked(prev.Format(DayLayout), []string{"eyes"}, 7); err != nil {
		t.Fatal(err)
	}
	m.rollDate(prev)
	if m.CheckedCount() != 1 {
		t.Errorf("should have loaded yesterday's check, got %d", m.CheckedCount())
	}
}

func TestTickMsgTriggersRolloverAndReschedules(t *testing.T) {
	m := testModel(t)
	tomorrow := time.Now().AddDate(0, 0, 1)
	next, cmd := m.Update(tickMsg(tomorrow))
	mm := next.(Model)
	if mm.date != tomorrow.Format(DayLayout) {
		t.Errorf("tick should roll the date, got %q", mm.date)
	}
	if cmd == nil {
		t.Error("tick must reschedule itself or rollover stops working")
	}
}

func TestToggleOutOfRangeIsIgnored(t *testing.T) {
	m := testModel(t)
	m = send(t, m, down())
	m.cursor = len(m.cfg.Items) // deliberately invalid
	before := m.hist.CheckedCount()
	m.toggle(m.cursor)
	if m.hist.CheckedCount() != before {
		t.Error("out-of-range toggle must be a no-op")
	}
}
