// Package classify раскладывает ветки по «мёртвости». Ни одной ссылки на git:
// всё, что нужно, приходит в структуре Branch — поэтому логика проверяется юнит-тестами.
package classify

import "time"

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
