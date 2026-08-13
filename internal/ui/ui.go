// Package ui — интерактивный выбор веток. Здесь только отображение и клавиши:
// какие ветки можно удалять, решает classify.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/art-ps/git-nanny/internal/classify"
)

var (
	dim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selected = lipgloss.NewStyle().Bold(true)
)

type model struct {
	entries   []classify.Entry
	checked   map[int]bool
	cursor    int
	confirm   bool
	done      bool
	cancelled bool
	now       time.Time
	force     bool
}

func Select(entries []classify.Entry, now time.Time, force bool) ([]classify.Entry, error) {
	m := model{entries: entries, checked: map[int]bool{}, now: now, force: force}
	for i, e := range entries {
		if e.Deletable(force) && (e.Category == classify.Merged || e.Category == classify.Gone) {
			m.checked[i] = true
		}
	}
	res, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	final := res.(model)
	if final.cancelled || !final.done {
		return nil, nil
	}
	var out []classify.Entry
	for i, e := range final.entries {
		if final.checked[i] {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.confirm {
		switch key.String() {
		case "y":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		default:
			m.confirm = false
			return m, nil
		}
	}
	switch key.String() {
	case "ctrl+c", "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case " ":
		if m.entries[m.cursor].Deletable(m.force) {
			m.checked[m.cursor] = !m.checked[m.cursor]
		}
	case "a":
		for i, e := range m.entries {
			if e.Deletable(m.force) {
				m.checked[i] = true
			}
		}
	case "n":
		m.checked = map[int]bool{}
	case "enter":
		if len(m.selectedNames()) > 0 {
			m.confirm = true
		}
	}
	return m, nil
}

func (m model) selectedNames() []string {
	var out []string
	for i, e := range m.entries {
		if m.checked[i] {
			out = append(out, e.Name)
		}
	}
	return out
}

func (m model) View() string {
	var b strings.Builder
	if m.confirm {
		names := m.selectedNames()
		b.WriteString(accent.Render(fmt.Sprintf("удалить %d веток?", len(names))) + "\n\n")
		for _, n := range names {
			b.WriteString("  " + n + "\n")
		}
		b.WriteString(dim.Render("\ny — удалить · любая другая клавиша — назад") + "\n")
		return b.String()
	}

	b.WriteString("нянька для веток · пробел — отметить · a — все · enter — удалить · q — выход\n\n")
	for i, e := range m.entries {
		cursor := "  "
		if i == m.cursor {
			cursor = "› "
		}
		box := "[ ]"
		if m.checked[i] {
			box = "[×]"
		}
		days := int(m.now.Sub(e.LastCommit).Hours() / 24)
		line := fmt.Sprintf("%s%s %s · %d дн · +%d/−%d · %s",
			cursor, box, e.Name, days, e.Ahead, e.Behind, e.Category.String())
		switch {
		case !e.Deletable(m.force):
			reason := e.ProtectReason
			if reason == "" {
				reason = "есть уникальные коммиты"
			}
			b.WriteString(dim.Render(fmt.Sprintf("%s    %s · %s", cursor, e.Name, reason)) + "\n")
			continue
		case i == m.cursor:
			b.WriteString(selected.Render(line) + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
