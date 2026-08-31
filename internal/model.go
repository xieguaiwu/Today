package internal

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// rolloverCheck is how often the UI re-reads the local date, so a terminal left
// open past midnight starts a fresh day instead of writing to yesterday's record.
const rolloverCheck = 30 * time.Second

// Views.
const (
	viewList = iota
	viewStats
)

type tickMsg time.Time

// Model is the TUI state for the daily self-check checklist.
type Model struct {
	cfg      Config
	hist     *History
	progress map[string]int // item id -> steps completed today
	cursor   int
	date     string
	err      error
	width    int

	view     int // viewList or viewStats
	statsSel int // -1 = 全部, otherwise index into cfg.Items
	year     int
}

// New builds a Model bound to a config and an open history file.
func New(cfg Config, hist *History) Model {
	date := time.Now().Format(DayLayout)
	return Model{
		cfg:      cfg,
		hist:     hist,
		progress: hist.ProgressSet(date),
		date:     date,
		view:     viewList,
		statsSel: -1,
		year:     time.Now().Year(),
	}
}

// Init starts the midnight-rollover ticker.
func (m Model) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(rolloverCheck, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update handles keyboard input and ticker messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.rollDate(time.Time(msg))
		return m, tick()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		// When several keystrokes arrive in one read (fast typing, or a paste),
		// bubbletea delivers them as a single KeyRunes message whose String() is
		// "jj" -- which matches no binding and would be dropped. Replay such a
		// burst one keystroke at a time so nothing is silently lost.
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
			cur := m
			for _, r := range msg.Runes {
				var cmd tea.Cmd
				cur, cmd = cur.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				if cmd != nil {
					return cur, cmd
				}
			}
			return cur, nil
		}
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey applies a single keystroke. The returned command is non-nil only
// for a quit request.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab", "esc":
		if m.view == viewList {
			m.view = viewStats
		} else {
			m.view = viewList
		}
		return m, nil
	}

	if m.view == viewStats {
		return m.handleStatsKey(msg)
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.cfg.Items)-1 {
			m.cursor++
		}
	case "enter", " ", "x":
		m.step(m.cursor)
	case "+", "=":
		m.addStep(m.cursor, +1)
	case "-", "_":
		m.addStep(m.cursor, -1)
	case "f":
		m.fill(m.cursor)
	case "r":
		m.resetDay()
	}
	return m, nil
}

// stepsOf is the configured step count for the item at idx.
func (m Model) stepsOf(idx int) int {
	if idx < 0 || idx >= len(m.cfg.Items) {
		return 1
	}
	return m.cfg.Items[idx].StepsOrDefault()
}

// step advances one sub-step, wrapping back to zero once the item is full. For
// a single-step item this is exactly the old check/uncheck toggle.
func (m *Model) step(idx int) {
	if idx < 0 || idx >= len(m.cfg.Items) {
		return
	}
	id := m.cfg.Items[idx].ID
	if m.progress[id] >= m.stepsOf(idx) {
		m.progress[id] = 0
	} else {
		m.progress[id]++
	}
	m.persist()
}

// addStep nudges an item by delta steps, clamped to [0, max].
func (m *Model) addStep(idx, delta int) {
	if idx < 0 || idx >= len(m.cfg.Items) {
		return
	}
	id := m.cfg.Items[idx].ID
	max := m.stepsOf(idx)
	v := m.progress[id] + delta
	if v < 0 {
		v = 0
	}
	if v > max {
		v = max
	}
	m.progress[id] = v
	m.persist()
}

// fill jumps straight to fully done, or clears if already full.
func (m *Model) fill(idx int) {
	if idx < 0 || idx >= len(m.cfg.Items) {
		return
	}
	id := m.cfg.Items[idx].ID
	if m.progress[id] >= m.stepsOf(idx) {
		m.progress[id] = 0
	} else {
		m.progress[id] = m.stepsOf(idx)
	}
	m.persist()
}

// handleStatsKey drives the statistics view: item picker, year stepper.
func (m Model) handleStatsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	options := len(m.cfg.Items) + 1 // + 全部
	switch msg.String() {
	case "left", "h", "p", "shift+tab":
		m.statsSel--
		if m.statsSel < -1 {
			m.statsSel = options - 2
		}
	case "right", "l", "n":
		m.statsSel++
		if m.statsSel > options-2 {
			m.statsSel = -1
		}
	case "up", "k":
		m.year--
	case "down", "j":
		m.year++
	}
	return m, nil
}

// selectedID is the history id being charted ("" = 全部).
func (m Model) selectedID() string {
	if m.statsSel < 0 || m.statsSel >= len(m.cfg.Items) {
		return AllItems
	}
	return m.cfg.Items[m.statsSel].ID
}

func (m Model) selectedColor() string {
	if m.statsSel < 0 || m.statsSel >= len(m.cfg.Items) {
		return "#A855F7" // 全部 uses purple, as in the source app
	}
	return m.cfg.Items[m.statsSel].ColorOrDefault(m.statsSel)
}

// rollDate adopts a new local day if the calendar date changed while open.
func (m *Model) rollDate(now time.Time) {
	next := now.Format(DayLayout)
	if next == m.date {
		return
	}
	m.date = next
	m.progress = m.hist.ProgressSet(next)
	m.cursor = 0
	m.err = nil
	m.year = now.Year()
}

func (m *Model) resetDay() {
	m.progress = map[string]int{}
	if err := m.hist.ResetDay(m.date); err != nil {
		m.err = err
		return
	}
	m.err = nil
}

// persist flushes today's step counts. Writes happen on every keystroke: the
// whole point of a daily record is that closing the terminal must not lose it.
func (m *Model) persist() {
	m.err = m.hist.SetProgress(m.date, m.progress, len(m.cfg.Items))
}

// CheckedCount is how many items are fully completed today.
func (m Model) CheckedCount() int {
	n := 0
	for _, it := range m.cfg.Items {
		if m.progress[it.ID] >= it.StepsOrDefault() {
			n++
		}
	}
	return n
}

// PartialCount is how many items have some, but not all, steps done today.
func (m Model) PartialCount() int {
	n := 0
	for _, it := range m.cfg.Items {
		v := m.progress[it.ID]
		if v > 0 && v < it.StepsOrDefault() {
			n++
		}
	}
	return n
}

// View renders the active screen.
func (m Model) View() string {
	if m.view == viewStats {
		return m.viewStats()
	}
	return m.viewList()
}

func (m Model) viewList() string {
	now := time.Now()
	var b strings.Builder

	b.WriteString(dim(fmt.Sprintf("%s · %s", m.cfg.Title, m.date)) + "\n")
	b.WriteString(fmt.Sprintf("%d/%d 已完成 · 全部连续 %d 天 · 累计 %d 天\n\n",
		m.CheckedCount(), len(m.cfg.Items),
		m.hist.ItemStreak(AllItems, now), m.hist.CheckedCount()))

	nameW := 0
	for _, it := range m.cfg.Items {
		if w := displayWidth(it.Label); w > nameW {
			nameW = w
		}
	}

	for i, it := range m.cfg.Items {
		cursor := "  "
		if m.cursor == i {
			cursor = fg(it.ColorOrDefault(i), "> ")
		}
		max := it.StepsOrDefault()
		done := m.progress[it.ID]
		if done > max {
			done = max
		}

		var mark string
		if max > 1 {
			// Multi-step habits get a bar instead of a checkbox.
			bar := strings.Repeat("▹", done) + strings.Repeat("▸", max-done)
			if done >= max {
				bar = strings.Repeat("▹", max)
			}
			mark = fg(it.ColorOrDefault(i), bar) + dim(fmt.Sprintf(" %d/%d", done, max))
		} else if done >= max {
			mark = fg(it.ColorOrDefault(i), "[x]")
		} else {
			mark = dim("[ ]")
		}
		label := it.Label
		pad := nameW - displayWidth(label)
		if pad < 0 {
			pad = 0
		}
		cells := weekCells(m.hist.WeekCells(it.ID, now), it.ColorOrDefault(i))
		streak := m.hist.ItemStreak(it.ID, now)
		b.WriteString(fmt.Sprintf("%s%s %s%s %s %s\n",
			cursor, mark, label, strings.Repeat(" ", pad), cells,
			dim(fmt.Sprintf("%d🔥", streak))))
		if m.cursor == i && it.Hint != "" {
			b.WriteString(dim(fmt.Sprintf("      %s\n", it.Hint)))
		}
	}

	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(dim("保存失败: "+m.err.Error()) + "\n")
	}
	if partial := m.PartialCount(); partial > 0 {
		b.WriteString(dim(fmt.Sprintf("%d 项部分完成（未计入连胜/完成次数）\n", partial)))
	}
	b.WriteString(dim("↑↓/jk 移动 · 空格 +1步 · f 拉满 · +/- 微调 · r 重置当日 · Tab 统计 · q 退出") + "\n")
	b.WriteString(dim("本周 一 二 三 四 五 六 日 · ■ 满 ▓ ░ 部分") + "\n")
	return b.String()
}

func (m Model) viewStats() string {
	now := time.Now()
	id := m.selectedID()
	color := m.selectedColor()
	title := "全部"
	if m.statsSel >= 0 && m.statsSel < len(m.cfg.Items) {
		title = m.cfg.Items[m.statsSel].Label
	}

	var b strings.Builder
	b.WriteString(dim("统计") + "  " + bold(fg(color, title)) + "\n")

	// picker
	parts := make([]string, 0, len(m.cfg.Items)+1)
	parts = append(parts, m.pickLabel("全部", -1, color))
	for i, it := range m.cfg.Items {
		parts = append(parts, m.pickLabel(it.Label, i, it.ColorOrDefault(i)))
	}
	b.WriteString(dim(strings.Join(parts, "  ")) + "\n\n")

	cw := 22
	cards := []string{
		statCard("🔥 当前连胜", fmt.Sprintf("%d 天", m.hist.ItemStreak(id, now)), "连续打卡", cw),
		statCard("🏆 最高连胜", fmt.Sprintf("%d 天", m.hist.ItemBest(id)), "记录内最佳", cw),
		statCard("✓ 完成次数", fmt.Sprintf("%d", m.hist.ItemCount(id)),
			fmt.Sprintf("%.1f 周均", m.hist.WeeklyAverage(id, now)), cw),
		statCard("🎯 持续", fmt.Sprintf("%.0f%%", m.hist.ItemRate(id, now, 90)*100), "最近 90 天", cw),
	}
	b.WriteString(statRow(cards) + "\n")

	b.WriteString(dim(fmt.Sprintf("打卡记录  %d  (↑↓ 换年)", m.year)) + "\n")
	grid, marks := m.hist.YearMatrix(id, m.year, now)
	maxCols := 52
	if m.width > 20 {
		maxCols = m.width - 6
	}
	b.WriteString(yearHeatmap(grid, marks, color, maxCols) + "\n")

	b.WriteString(dim("你在什么时候最能持之以恒？") + "\n")
	b.WriteString(weekdayChart(m.hist.WeekdayHistogram(id), color))

	b.WriteString("\n")
	b.WriteString(dim("←→/hl 换条目 · ↑↓ 换年 · Tab 返回 · q 退出") + "\n")
	return b.String()
}

// pickLabel renders one chip of the stats picker; the selected one is boxed
// and coloured, the rest are dimmed with their own colour dot.
func (m Model) pickLabel(label string, sel int, color string) string {
	dot := ""
	if sel >= 0 && sel < len(m.cfg.Items) {
		dot = fg(color, "●") + " "
	}
	if m.statsSel == sel {
		return bold(fg(color, "["+label+"]"))
	}
	return dim(dot + label)
}
