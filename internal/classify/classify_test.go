package classify

import (
	"testing"
	"time"
)

func at(daysAgo int) time.Time {
	return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo)
}

var now = at(0)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		b    Branch
		want Category
	}{
		{"вмёржена — всегда merged", Branch{Merged: true, LastCommit: at(1)}, Merged},
		{"вмёржена и upstream удалён — merged важнее", Branch{Merged: true, UpstreamGone: true, LastCommit: at(1)}, Merged},
		{"upstream удалён", Branch{UpstreamGone: true, LastCommit: at(1)}, Gone},
		{"старая без upstream", Branch{LastCommit: at(91), Ahead: 3}, Stale},
		{"ровно на пороге — ещё не stale", Branch{LastCommit: at(90), Ahead: 3}, Active},
		{"свежая с работой", Branch{LastCommit: at(2), Ahead: 3}, Active},
	}
	for _, c := range cases {
		if got := Classify(c.b, now, 90); got != c.want {
			t.Errorf("%s: получили %v, ждали %v", c.name, got, c.want)
		}
	}
}

func TestCategoryOrder(t *testing.T) {
	if !(Merged < Gone && Gone < Stale && Stale < Active) {
		t.Fatal("порядок категорий задаёт порядок сортировки и не должен меняться")
	}
}
