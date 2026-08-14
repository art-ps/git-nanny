package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/art-ps/git-nanny/internal/classify"
	"github.com/art-ps/git-nanny/internal/gitrepo"
	"github.com/art-ps/git-nanny/internal/journal"
)

// gitTestRepo — минимальный помощник для тестов Run: временный репозиторий и
// команда git внутри него.
func gitTestRepo(t *testing.T) (dir string, git func(args ...string)) {
	t.Helper()
	dir = t.TempDir()
	git = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir, git
}

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
		Branch: classify.Branch{
			Name: "feature/x", LastCommit: day(94), Behind: 128,
			Created: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		},
		Category: classify.Merged,
	}
	s := FormatEntry(e, day(0))
	for _, want := range []string{"feature/x", "94 days ago", "−128", "merged"} {
		if !strings.Contains(s, want) {
			t.Errorf("в строке %q нет %q", s, want)
		}
	}
	if !strings.HasSuffix(s, "created 2026-05-04") {
		t.Errorf("строка %q не оканчивается датой создания", s)
	}
}

func TestPlanWithoutScopeIsEmpty(t *testing.T) {
	if got := Plan(entries(), Options{}); len(got) != 0 {
		t.Fatalf("без области действия план должен быть пуст, получили %v", names(got))
	}
	if got := Plan(entries(), Options{Force: true}); len(got) != 0 {
		t.Fatalf("--force сам по себе не задаёт область, получили %v", names(got))
	}
	if got := Plan(entries(), Options{DryRun: true}); len(got) != 0 {
		t.Fatalf("--dry-run сам по себе не задаёт область, получили %v", names(got))
	}
}

func TestRunSkipsDeletionWhenJournalFails(t *testing.T) {
	// XDG_STATE_HOME указывает на файл — MkdirAll под него не сработает
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocker)

	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main", ".")
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("checkout", "-q", "-b", "merged")
	git("commit", "-q", "--allow-empty", "-m", "x")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "m", "merged")

	var buf bytes.Buffer
	code, err := Run(dir, Options{Merged: true, Yes: true, StaleDays: 90}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 1 {
		t.Errorf("код возврата %d, ждали 1", code)
	}
	if !strings.Contains(buf.String(), "could not write the journal") {
		t.Errorf("нет сообщения о сбое журнала: %s", buf.String())
	}
	out, _ := exec.Command("git", "-C", dir, "branch", "--list", "merged").Output()
	if !strings.Contains(string(out), "merged") {
		t.Error("ветка удалена, хотя журнал не записался — восстановить её нечем")
	}
}

// TestRunDeletesSquashMergedBranch — регрессия на главный баг финального ревью:
// `--merged --yes` должен уметь удалить ветку, влитую сквошем (`git branch -d`
// откажет, т.к. коммит ветки не предок main; нужен force именно для неё).
func TestRunDeletesSquashMergedBranch(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir, git := gitTestRepo(t)

	git("init", "-q", "-b", "main", ".")
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("checkout", "-q", "-b", "squashed")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-q", "-m", "add a")
	git("checkout", "-q", "main")
	git("merge", "-q", "--squash", "squashed")
	git("commit", "-q", "-m", "squash squashed")

	var buf bytes.Buffer
	code, err := Run(dir, Options{Merged: true, Yes: true, StaleDays: 90}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("код возврата %d, ждали 0\n%s", code, buf.String())
	}
	out, _ := exec.Command("git", "-C", dir, "branch", "--list", "squashed").Output()
	if strings.Contains(string(out), "squashed") {
		t.Errorf("сквош-вмёрженная ветка не удалена:\n%s", buf.String())
	}
}

// TestRunNoDeleteWhenDefaultBranchUnknown: репозиторий "git init -b develop" с
// двумя ветками, без remote, стоя не на develop. Основную ветку определить
// нельзя — Run обязан ничего не удалять и объяснить, как её задать.
func TestRunNoDeleteWhenDefaultBranchUnknown(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir, git := gitTestRepo(t)

	git("init", "-q", "-b", "develop", ".")
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("checkout", "-q", "-b", "feature")
	git("commit", "-q", "--allow-empty", "-m", "x")

	var buf bytes.Buffer
	code, err := Run(dir, Options{Merged: true, Yes: true, StaleDays: 90}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 1 {
		t.Errorf("код возврата %d, ждали 1", code)
	}
	if !strings.Contains(buf.String(), "default branch") {
		t.Errorf("нет объяснения про основную ветку: %s", buf.String())
	}
	for _, name := range []string{"develop", "feature"} {
		out, _ := exec.Command("git", "-C", dir, "branch", "--list", name).Output()
		if !strings.Contains(string(out), name) {
			t.Errorf("ветка %s пропала, хотя основную ветку не определили", name)
		}
	}
}

// TestJournalFoundFromSubdirectory: запись об удалении, сделанная из подкаталога
// репозитория, должна находиться journal.ForRepo по корню репозитория.
func TestJournalFoundFromSubdirectory(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir, git := gitTestRepo(t)

	git("init", "-q", "-b", "main", ".")
	git("commit", "-q", "--allow-empty", "-m", "init")
	git("checkout", "-q", "-b", "merged")
	git("commit", "-q", "--allow-empty", "-m", "x")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "m", "merged")

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	code, err := Run(sub, Options{Merged: true, Yes: true, StaleDays: 90}, &buf)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("код возврата %d, ждали 0\n%s", code, buf.String())
	}

	repo, err := gitrepo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := journal.ForRepo(repo.Dir(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Branch != "merged" {
		t.Fatalf("запись об удалении из подкаталога не найдена по корню: %+v", recs)
	}
}
