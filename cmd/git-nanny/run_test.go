package main

import (
	"strings"
	"testing"
	"time"

	"github.com/art-ps/git-nanny/internal/classify"
)

func day(n int) time.Time {
	return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC).AddDate(0, 0, -n)
}

func entries() []classify.Entry {
	return classify.Build([]classify.Branch{
		{Name: "main", LastCommit: day(1)},
		{Name: "cur", LastCommit: day(1), Current: true},
		{Name: "merged-old", LastCommit: day(200), Merged: true},
		{Name: "gone-one", LastCommit: day(10), UpstreamGone: true, Ahead: 2},
		{Name: "stale-wip", LastCommit: day(300), Ahead: 5},
		{Name: "active", LastCommit: day(2), Ahead: 1},
	}, "main", nil, day(0), 90)
}

func names(es []classify.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Name
	}
	return out
}

func TestPlanMergedOnly(t *testing.T) {
	got := names(Plan(entries(), Options{Merged: true}))
	want := []string{"merged-old"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("получили %v, ждали %v", got, want)
	}
}

func TestPlanAllButDefaultSkipsProtectedAndUnique(t *testing.T) {
	got := names(Plan(entries(), Options{AllButDefault: true}))
	// main и cur защищены; gone-one и stale-wip имеют уникальные коммиты без force
	want := []string{"merged-old"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("получили %v, ждали %v", got, want)
	}
}

func TestPlanAllButDefaultWithForce(t *testing.T) {
	got := names(Plan(entries(), Options{AllButDefault: true, Force: true}))
	want := map[string]bool{"merged-old": true, "gone-one": true, "stale-wip": true, "active": true}
	if len(got) != len(want) {
		t.Fatalf("получили %v, ждали %d веток", got, len(want))
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("лишняя ветка в плане: %s", n)
		}
		if n == "main" || n == "cur" {
			t.Errorf("защищённая ветка попала в план: %s", n)
		}
	}
}

func TestFormatEntryShowsReasonAndAge(t *testing.T) {
	e := classify.Entry{
		Branch:   classify.Branch{Name: "feature/x", LastCommit: day(94), Behind: 128},
		Category: classify.Merged,
	}
	s := FormatEntry(e, day(0))
	for _, want := range []string{"feature/x", "94 дня назад", "−128", "вмёржена"} {
		if !strings.Contains(s, want) {
			t.Errorf("в строке %q нет %q", s, want)
		}
	}
}
