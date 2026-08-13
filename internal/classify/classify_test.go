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

func TestBuildProtects(t *testing.T) {
	in := []Branch{
		{Name: "main", LastCommit: at(1)},
		{Name: "feature/x", LastCommit: at(1), Current: true},
		{Name: "hotfix", LastCommit: at(1), InWorktree: true},
		{Name: "release/1.2", LastCommit: at(1), Merged: true},
		{Name: "old", LastCommit: at(200), Merged: true},
	}
	got := Build(in, "main", []string{"release/*"}, now, 90)

	byName := map[string]Entry{}
	for _, e := range got {
		byName[e.Name] = e
	}
	for name, reason := range map[string]string{
		"main":        "default",
		"feature/x":   "current",
		"hotfix":      "in use by a worktree",
		"release/1.2": "protected by pattern release/*",
	} {
		if !byName[name].Protected {
			t.Errorf("%s должна быть защищена", name)
		}
		if byName[name].ProtectReason != reason {
			t.Errorf("%s: причина %q, ждали %q", name, byName[name].ProtectReason, reason)
		}
		if byName[name].Deletable(true) {
			t.Errorf("%s не должна удаляться даже с force", name)
		}
	}
	if !byName["old"].Deletable(false) {
		t.Error("вмёрженная незащищённая ветка должна удаляться")
	}
}

// TestBuildProtectMatchesAncestors: path.Match не пропускает '*' через '/',
// поэтому nanny.protect 'release/*' сам по себе не защитит release/v2/hotfix.
// Проверяем шаблон и против имени, и против всех его предков по '/'.
func TestBuildProtectMatchesAncestors(t *testing.T) {
	in := []Branch{
		{Name: "release/v2/hotfix", LastCommit: at(1)},
	}
	got := Build(in, "main", []string{"release/*"}, now, 90)
	if !got[0].Protected {
		t.Fatal("release/v2/hotfix должна быть защищена шаблоном release/* через предка release/v2")
	}
	if got[0].ProtectReason != "protected by pattern release/*" {
		t.Errorf("причина %q неожиданная", got[0].ProtectReason)
	}
}

func TestBuildSorts(t *testing.T) {
	in := []Branch{
		{Name: "active-new", LastCommit: at(1)},
		{Name: "stale-old", LastCommit: at(300)},
		{Name: "merged-new", LastCommit: at(2), Merged: true},
		{Name: "merged-old", LastCommit: at(200), Merged: true},
		{Name: "gone", LastCommit: at(5), UpstreamGone: true},
	}
	got := Build(in, "main", nil, now, 90)
	want := []string{"merged-old", "merged-new", "gone", "stale-old", "active-new"}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("позиция %d: %s, ждали %s", i, got[i].Name, name)
		}
	}
}

func TestDeletableNeedsForceForUniqueCommits(t *testing.T) {
	e := Entry{Branch: Branch{Name: "wip", Ahead: 3, LastCommit: at(200)}, Category: Stale}
	if e.Deletable(false) {
		t.Error("ветка с уникальными коммитами не должна удаляться без force")
	}
	if !e.Deletable(true) {
		t.Error("с force должна")
	}
}
