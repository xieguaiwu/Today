package internal

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the TUI application state for the multi-select checklist.
type Model struct {
	cursor   int
	items    []string
	selected map[int]struct{}
}

// PersonData returns a new Model initialized with the default checklist items.
func PersonData() Model {
	items := []string{"Eyes", "Nose", "Skin", "Lips", "Anxiety", "Cognition", "Weight & Fat"}
	return Model{
		items:    items,
		selected: make(map[int]struct{}, len(items)),
	}
}

// Init returns nil as no initial command is needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input and returns the updated model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", " ":
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}
	return m, nil
}

// View renders the checklist UI as a string for the TUI.
func (m Model) View() string {
	s := "What to buy?\n\n"
	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "x"
		}
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, item)
	}
	s += "\nPress q to quit.\n"
	return s
}

