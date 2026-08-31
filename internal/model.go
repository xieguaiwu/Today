package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// rolloverCheck is how often the UI re-reads the local date, so a terminal left
// open past midnight starts a fresh day instead of writing to yesterday's record.
const rolloverCheck = 30 * time.Second

type tickMsg time.Time

// Model is the TUI state for the daily self-check checklist.
type Model struct {
	cfg     Config
	hist    *History
	checked map[string]struct{}
	cursor  int
	date    string
	err     error
	width   int
}

// New builds a Model bound to a config and an open history file.
func New(cfg Config, hist *History) Model {
	date := time.Now().Format(DayLayout)
	return Model{
		cfg:     cfg,
		hist:    hist,
		checked: hist.CheckedSet(date),
		date:    date,
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

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.cfg.Items)-1 {
			m.cursor++
		}

	case "enter", " ", "x":
		m.toggle(m.cursor)

	case "r":
		m.resetDay()
	}
	return m, nil
}

// rollDate adopts a new local day if the calendar date changed while open.
func (m *Model) rollDate(now time.Time) {
	next := now.Format(DayLayout)
	if next == m.date {
		return
	}
	m.date = next
	m.checked = m.hist.CheckedSet(next)
	m.cursor = 0
	m.err = nil
}

func (m *Model) toggle(idx int) {
	if idx < 0 || idx >= len(m.cfg.Items) {
		return
	}
	id := m.cfg.Items[idx].ID
	if _, ok := m.checked[id]; ok {
		delete(m.checked, id)
	} else {
		m.checked[id] = struct{}{}
	}
	m.persist()
}

func (m *Model) resetDay() {
	m.checked = map[string]struct{}{}
	if err := m.hist.ResetDay(m.date); err != nil {
		m.err = err
		return
	}
	m.err = nil
}

// persist flushes the current selection. Toggling writes immediately: the whole
// point of a daily record is that closing the terminal must not lose it.
func (m *Model) persist() {
	ids := make([]string, 0, len(m.checked))
	for _, it := range m.cfg.Items {
		if _, ok := m.checked[it.ID]; ok {
			ids = append(ids, it.ID)
		}
	}
	m.err = m.hist.SetChecked(m.date, ids, len(m.cfg.Items))
}

// CheckedCount is how many items are ticked today.
func (m Model) CheckedCount() int {
	n := 0
	for _, it := range m.cfg.Items {
		if _, ok := m.checked[it.ID]; ok {
			n++
		}
	}
	return n
}

// View renders the checklist.
func (m Model) View() string {
	total := len(m.cfg.Items)
	done := m.CheckedCount()

	var b strings.Builder
	b.WriteString(dim(fmt.Sprintf("%s · %s", m.cfg.Title, m.date)) + "\n")
	b.WriteString(fmt.Sprintf("%d/%d 已完成 · 连续打卡 %d 天 · 累计打卡 %d 天\n\n",
		done, total, m.hist.Streak(time.Now()), m.hist.CheckedCount()))

	for i, it := range m.cfg.Items {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		mark := " "
		if _, ok := m.checked[it.ID]; ok {
			mark = "x"
		}
		b.WriteString(fmt.Sprintf("%s[%s] %s\n", cursor, mark, it.Label))
		if m.cursor == i && it.Hint != "" {
			b.WriteString(dim(fmt.Sprintf("    %s\n", it.Hint)))
		}
	}

	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(dim("保存失败: "+m.err.Error()) + "\n")
	}
	b.WriteString(dim("↑↓/jk 移动 · 空格 勾选 · r 重置当日 · q 退出") + "\n")
	return b.String()
}

// dim wraps text in the ANSI "faint" sequence unless color is disabled.
func dim(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}
