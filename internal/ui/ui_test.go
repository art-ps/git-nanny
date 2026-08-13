package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/art-ps/git-nanny/internal/classify"
)

// TestProtectedEntryCannotBeChecked: защищённую запись нельзя отметить ни
// пробелом, ни клавишей "a" (отметить всё) — что бы ни делал пользователь,
// Deletable(force) остаётся false для неё.
func TestProtectedEntryCannotBeChecked(t *testing.T) {
	now := time.Now()
	entries := []classify.Entry{
		{
			Branch:    classify.Branch{Name: "main", LastCommit: now},
			Category:  classify.Active,
			Protected: true, ProtectReason: "default",
		},
	}
	m := newModel(entries, now, false)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(model)
	if m.checked[0] {
		t.Fatal("защищённую запись отметило пробелом")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(model)
	if m.checked[0] {
		t.Fatal("защищённую запись отметило клавишей a (отметить всё)")
	}
}
