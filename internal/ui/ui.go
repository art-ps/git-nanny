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
}

func newModel(entries []classify.Entry, now time.Time) model {
	m := model{entries: entries, checked: map[int]bool{}, now: now}
	for i, e := range entries {
		if preselectable(e) {
			m.checked[i] = true
		}
	}
	return m
}

// preselectable — набор веток, отмечаемых по умолчанию и клавишей "m":
// смёрженные и с ушедшим апстримом, если ветка не защищена. Уникальные
// коммиты здесь не проверяются — выбор консервативен по категории, а не
// по Deletable: интерактивный список решает сам, non-interactive путь
// по-прежнему гейтится Deletable(force) в cmd/git-nanny/run.go.
func preselectable(e classify.Entry) bool {
	return !e.Protected && (e.Category == classify.Merged || e.Category == classify.Gone)
}

func Select(entries []classify.Entry, now time.Time, force bool) ([]classify.Entry, error) {
	m := newModel(entries, now)
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
		if !m.entries[m.cursor].Protected {
			m.checked[m.cursor] = !m.checked[m.cursor]
		}
	case "a":
		for i, e := range m.entries {
			if !e.Protected {
				m.checked[i] = true
			}
		}
	case "m":
		for i, e := range m.entries {
			if preselectable(e) {
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
		word := "branches"
		if len(names) == 1 {
			word = "branch"
		}
		b.WriteString(accent.Render(fmt.Sprintf("delete %d %s?", len(names), word)) + "\n\n")
		for _, n := range names {
			b.WriteString("  " + n + "\n")
		}
		if unique := m.selectedUniqueCommits(); unique > 0 {
			verb, be := "have", "are"
			if unique == 1 {
				verb, be = "has", "is"
			}
			b.WriteString(fmt.Sprintf("\n%d of them %s unique commits and %s not merged anywhere\n", unique, verb, be))
		}
		b.WriteString(dim.Render("\ny — delete · any other key — back") + "\n")
		return b.String()
	}

	b.WriteString("branch nanny · space — toggle · a — all · m — merged only · enter — delete · q — quit\n\n")
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
		created := e.Created.Format("2006-01-02")

		if e.Protected {
			reason := e.ProtectReason
			if reason == "" {
				reason = "protected"
			}
			b.WriteString(dim.Render(fmt.Sprintf("%s    %s · %s · created %s", cursor, e.Name, reason, created)) + "\n")
			continue
		}

		category := e.Category.String()
		if e.Category != classify.Merged && e.Ahead > 0 {
			category += " · has unique commits"
		}
		line := fmt.Sprintf("%s%s %s · %d d · +%d/−%d · %s · created %s",
			cursor, box, e.Name, days, e.Ahead, e.Behind, category, created)
		if i == m.cursor {
			b.WriteString(selected.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// selectedUniqueCommits считает отмеченные ветки, у которых есть коммиты,
// не попавшие в default branch, и которые не помечены merged — для строки
// на экране подтверждения.
func (m model) selectedUniqueCommits() int {
	n := 0
	for i, e := range m.entries {
		if m.checked[i] && e.Category != classify.Merged && e.Ahead > 0 {
			n++
		}
	}
	return n
}
