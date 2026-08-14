package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/art-ps/git-nanny/internal/classify"
)

// TestProtectedEntryCannotBeChecked: защищённую запись нельзя отметить ни
// пробелом, ни клавишей "a" (отметить всё) — что бы ни делал пользователь,
// e.Protected остаётся true, и оба гейта её пропускают.
func TestProtectedEntryCannotBeChecked(t *testing.T) {
	now := time.Now()
	entries := []classify.Entry{
		{
			Branch:    classify.Branch{Name: "main", LastCommit: now},
			Category:  classify.Active,
			Protected: true, ProtectReason: "default",
		},
	}
	m := newModel(entries, now)

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

// TestUniqueCommitsEntryCanBeCheckedBySpace: незащищённую ветку с уникальными
// коммитами (Ahead > 0, не merged) можно отметить пробелом — регрессия из
// реального запуска, где почти все ветки были такими и список был бесполезен.
func TestUniqueCommitsEntryCanBeCheckedBySpace(t *testing.T) {
	now := time.Now()
	entries := []classify.Entry{
		{
			Branch:   classify.Branch{Name: "feature/x", LastCommit: now, Ahead: 3},
			Category: classify.Active,
		},
	}
	m := newModel(entries, now)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(model)
	if !m.checked[0] {
		t.Fatal("незащищённую запись с уникальными коммитами не отметило пробелом")
	}
}

// TestSelectAllChecksUniqueCommitsButNotProtected: "a" отмечает ветку с
// уникальными коммитами (незащищённую), но не трогает защищённую.
func TestSelectAllChecksUniqueCommitsButNotProtected(t *testing.T) {
	now := time.Now()
	entries := []classify.Entry{
		{
			Branch:   classify.Branch{Name: "feature/x", LastCommit: now, Ahead: 3},
			Category: classify.Active,
		},
		{
			Branch:    classify.Branch{Name: "main", LastCommit: now},
			Category:  classify.Active,
			Protected: true, ProtectReason: "default",
		},
	}
	m := newModel(entries, now)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(model)
	if !m.checked[0] {
		t.Fatal("клавиша a не отметила незащищённую ветку с уникальными коммитами")
	}
	if m.checked[1] {
		t.Fatal("клавиша a отметила защищённую ветку")
	}
}

// TestMergedOnlyKeyChecksMergedNotUnique: "m" отмечает merged-ветку, но не
// трогает незащищённую ветку с уникальными коммитами. Предвыбор в newModel
// уже отмечает запись 0 (Category: Merged), а неизвестная клавиша в Update —
// no-op, так что без явного снятия предвыбора клавишей "n" тест прошёл бы и
// без кейса "m" в Update. Снимаем всё "n", проверяем, что ничего не осталось
// отмеченным, и только потом жмём "m" — так отметка может появиться только
// от неё.
func TestMergedOnlyKeyChecksMergedNotUnique(t *testing.T) {
	now := time.Now()
	entries := []classify.Entry{
		{
			Branch:   classify.Branch{Name: "done", LastCommit: now},
			Category: classify.Merged,
		},
		{
			Branch:   classify.Branch{Name: "feature/x", LastCommit: now, Ahead: 3},
			Category: classify.Active,
		},
	}
	m := newModel(entries, now)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(model)
	if len(m.selectedNames()) != 0 {
		t.Fatalf("после n не должно остаться отметок: %v", m.selectedNames())
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = next.(model)
	if !m.checked[0] {
		t.Fatal("клавиша m не отметила merged-ветку")
	}
	if m.checked[1] {
		t.Fatal("клавиша m отметила ветку с уникальными коммитами")
	}
}
