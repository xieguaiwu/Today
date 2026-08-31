package internal

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabTogglesBetweenViews(t *testing.T) {
	m := testModel(t)
	if m.view != viewList {
		t.Fatalf("should start on the list view")
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.view != viewStats {
		t.Fatal("Tab should open stats")
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.view != viewList {
		t.Fatal("Tab should return to list")
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.view != viewStats {
		t.Fatal("Esc/Tab should open stats again")
	}
}

func TestQuitWorksFromBothViews(t *testing.T) {
	m := testModel(t)
	if _, cmd := m.Update(key('q')); cmd == nil {
		t.Error("q should quit from list")
	}
	stats := send(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if _, cmd := stats.Update(key('q')); cmd == nil {
		t.Error("q should quit from stats too")
	}
}

func TestStatsPickerWrapsAround(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	n := len(m.cfg.Items)

	if m.statsSel != -1 {
		t.Fatalf("stats should open on 全部, got %d", m.statsSel)
	}
	// right once -> first item
	m = send(t, m, key('l'))
	if m.statsSel != 0 {
		t.Errorf("after l, sel = %d, want 0", m.statsSel)
	}
	// left back -> 全部
	m = send(t, m, key('h'))
	if m.statsSel != -1 {
		t.Errorf("after h, sel = %d, want -1", m.statsSel)
	}
	// left again -> last item (wrap)
	m = send(t, m, key('h'))
	if m.statsSel != n-1 {
		t.Errorf("wrap backwards sel = %d, want %d", m.statsSel, n-1)
	}
	// right -> 全部 again
	m = send(t, m, key('l'))
	if m.statsSel != -1 {
		t.Errorf("wrap forwards sel = %d, want -1", m.statsSel)
	}
}

func TestSelectedIDAndColor(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	if got := m.selectedID(); got != AllItems {
		t.Errorf("全部 id = %q, want empty", got)
	}
	if got := m.selectedColor(); got != "#A855F7" {
		t.Errorf("全部 color = %q, want purple", got)
	}
	m = send(t, m, key('l')) // first item = anki单词
	if got := m.selectedID(); got != "anki" {
		t.Errorf("id = %q, want anki", got)
	}
	if got := m.selectedColor(); got != "#3B82F6" {
		t.Errorf("color = %q, want the configured blue", got)
	}
}

func TestStatsArrowsChangeYearNotCursor(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	cursor := m.cursor
	year := m.year

	m = send(t, m, up())
	if m.year != year-1 {
		t.Errorf("up should step the year back, got %d", m.year)
	}
	if m.cursor != cursor {
		t.Errorf("cursor must not move in stats view: %d", m.cursor)
	}
	m = send(t, m, down())
	if m.year != year {
		t.Errorf("down should restore the year, got %d", m.year)
	}
}

func TestListArrowsStillMoveCursor(t *testing.T) {
	m := testModel(t)
	m = send(t, m, down())
	if m.cursor != 1 {
		t.Errorf("list view cursor should move, got %d", m.cursor)
	}
}

func TestSpaceDoesNotToggleInStatsView(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	m = send(t, m, space())
	if m.CheckedCount() != 0 {
		t.Error("space must not check items while viewing stats")
	}
}

func TestStatsViewRendersAllSections(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	v := stripANSI(m.View())

	for _, want := range []string{
		"统计", "全部", "anki单词",
		"当前连胜", "最高连胜", "完成次数", "持续",
		"打卡记录", "你在什么时候最能持之以恒？",
		"周一", "周日",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("stats view missing %q:\n%s", want, v)
		}
	}
	for _, bad := range []string{"What to buy?", "Press q to quit."} {
		if strings.Contains(v, bad) {
			t.Errorf("tutorial leftover %q still present", bad)
		}
	}
}

func TestListViewShowsWeekCellsAndStreak(t *testing.T) {
	m := testModel(t)
	m = send(t, m, space()) // check the first habit today
	v := stripANSI(m.View())

	if !strings.Contains(v, cellToday) {
		t.Errorf("today's cell should use the today glyph:\n%s", v)
	}
	if !strings.Contains(v, "1🔥") {
		t.Errorf("per-item streak count missing:\n%s", v)
	}
	if !strings.Contains(v, "本周") {
		t.Error("week legend missing")
	}
}

func TestRolloverResetsYearToCurrent(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	m.year = 2020
	m.rollDate(time.Now().AddDate(0, 0, 1))
	if m.year == 2020 {
		t.Error("a new day should bring the year view back to the present")
	}
}

func TestStatsViewIsBoundedWhenHistoryEmpty(t *testing.T) {
	m := send(t, testModel(t), tea.KeyMsg{Type: tea.KeyTab})
	v := stripANSI(m.View())
	if strings.Contains(v, "NaN") || strings.Contains(v, "0x") {
		t.Errorf("empty history produced junk:\n%s", v)
	}
	// 0% and 0 天 are legitimate; the heatmap should say so rather than crash.
	if !strings.Contains(v, "0 天") {
		t.Errorf("expected zero streaks on empty history:\n%s", v)
	}
}
