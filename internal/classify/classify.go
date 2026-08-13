// Package classify раскладывает ветки по «мёртвости». Ни одной ссылки на git:
// всё, что нужно, приходит в структуре Branch — поэтому логика проверяется юнит-тестами.
package classify

import (
	"path"
	"sort"
	"strings"
	"time"
)

type Category int

const (
	Merged Category = iota // порядок констант = порядок сортировки
	Gone
	Stale
	Active
)

func (c Category) String() string {
	switch c {
	case Merged:
		return "вмёржена"
	case Gone:
		return "upstream удалён"
	case Stale:
		return "давно не трогали"
	default:
		return "активная"
	}
}

type Branch struct {
	Name         string
	Head         string
	LastCommit   time.Time
	Ahead        int // уникальных коммитов относительно основной ветки
	Behind       int
	UpstreamGone bool
	Merged       bool // включая сквош-мерж
	SquashMerged bool // содержимое влито сквошем: коммит ветки не предок основной, но `git branch -d` откажет
	Current      bool
	InWorktree   bool
}

func Classify(b Branch, now time.Time, staleDays int) Category {
	switch {
	case b.Merged:
		return Merged
	case b.UpstreamGone:
		return Gone
	case now.Sub(b.LastCommit) > time.Duration(staleDays)*24*time.Hour:
		return Stale
	default:
		return Active
	}
}

type Entry struct {
	Branch
	Category      Category
	Protected     bool
	ProtectReason string
}

func Build(branches []Branch, defaultBranch string, protect []string, now time.Time, staleDays int) []Entry {
	out := make([]Entry, 0, len(branches))
	for _, b := range branches {
		e := Entry{Branch: b, Category: Classify(b, now, staleDays)}
		switch {
		case b.Current:
			e.Protected, e.ProtectReason = true, "текущая"
		case b.Name == defaultBranch:
			e.Protected, e.ProtectReason = true, "основная"
		case b.InWorktree:
			e.Protected, e.ProtectReason = true, "занята worktree"
		default:
			for _, pat := range protect {
				if matchesProtect(pat, b.Name) {
					e.Protected, e.ProtectReason = true, "защищена шаблоном "+pat
					break
				}
			}
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if !out[i].LastCommit.Equal(out[j].LastCommit) {
			return out[i].LastCommit.Before(out[j].LastCommit)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// matchesProtect проверяет шаблон против полного имени ветки и против всех его
// предков по «/»: nanny.protect 'release/*' должен защищать и release/v2/hotfix,
// а не только release/v2 — path.Match не пропускает '*' через '/'.
func matchesProtect(pat, name string) bool {
	for n := name; ; {
		if ok, err := path.Match(pat, n); err == nil && ok {
			return true
		}
		i := strings.LastIndex(n, "/")
		if i < 0 {
			return false
		}
		n = n[:i]
	}
}

func (e Entry) Deletable(force bool) bool {
	if e.Protected {
		return false
	}
	if e.Category != Merged && e.Ahead > 0 && !force {
		return false
	}
	return true
}
