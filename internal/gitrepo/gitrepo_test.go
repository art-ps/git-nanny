package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultBranchFallsBackToMain(t *testing.T) {
	dir := newTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.DefaultBranch(); got != "main" {
		t.Fatalf("основная ветка %q, ждали main", got)
	}
}

// TestDefaultBranchEmptyWhenAmbiguous: репозиторий "git init -b develop" без
// remote, с двумя ветками, ни main, ни master. Текущая ветка не должна
// подставляться как основная — иначе develop (не текущая) окажется кандидатом
// на удаление без --force.
func TestDefaultBranchEmptyWhenAmbiguous(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "develop")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "x")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.DefaultBranch(); got != "" {
		t.Fatalf("основная ветка %q, ждали пустую строку (неопределённость)", got)
	}
}

// TestDefaultBranchHonorsConfig: nanny.defaultBranch — явная воля пользователя,
// побеждает всё остальное, включая неоднозначность из теста выше.
func TestDefaultBranchHonorsConfig(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "develop")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "x")
	gitIn(t, dir, "config", "nanny.defaultBranch", "develop")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.DefaultBranch(); got != "develop" {
		t.Fatalf("основная ветка %q, ждали develop", got)
	}
}

// TestOpenResolvesToRepoRoot: журнал должен находиться независимо от подкаталога,
// из которого запущена утилита — ключом служит корень репозитория.
func TestOpenResolvesToRepoRoot(t *testing.T) {
	dir := newTestRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	fromRoot, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	fromSub, err := Open(sub)
	if err != nil {
		t.Fatal(err)
	}
	if fromRoot.Dir() != fromSub.Dir() {
		t.Fatalf("корень из подкаталога %q, из корня %q — не совпадают", fromSub.Dir(), fromRoot.Dir())
	}
}

func TestOpenRejectsNonRepo(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("ждали ошибку на каталоге без git")
	}
}

func TestBranchesDetectsMergeKinds(t *testing.T) {
	dir := newTestRepo(t)

	// обычный мерж
	gitIn(t, dir, "checkout", "-q", "-b", "plain")
	commitFile(t, dir, "plain.txt", "1")
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "-m", "merge plain", "plain")

	// сквош-мерж: содержимое в main, коммит ветки не является предком
	gitIn(t, dir, "checkout", "-q", "-b", "squashed")
	commitFile(t, dir, "squashed.txt", "a")
	commitFile(t, dir, "squashed2.txt", "b")
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--squash", "squashed")
	gitIn(t, dir, "commit", "-q", "-m", "squash squashed")

	// живая ветка с уникальной работой
	gitIn(t, dir, "checkout", "-q", "-b", "alive")
	commitFile(t, dir, "alive.txt", "x")
	gitIn(t, dir, "checkout", "-q", "main")

	r, _ := Open(dir)
	bs, err := r.Branches("main")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	ahead := map[string]int{}
	for _, b := range bs {
		got[b.Name] = b.Merged
		ahead[b.Name] = b.Ahead
	}
	if !got["plain"] {
		t.Error("обычный мерж не распознан")
	}
	if !got["squashed"] {
		t.Error("сквош-мерж не распознан — это главная функция утилиты")
	}
	if got["alive"] {
		t.Error("живая ветка помечена как вмёрженная")
	}
	if ahead["alive"] != 1 {
		t.Errorf("alive: ahead=%d, ждали 1", ahead["alive"])
	}
	if ahead["plain"] != 0 {
		t.Errorf("plain: ahead=%d, ждали 0", ahead["plain"])
	}
}

func TestBranchesMarksCurrentAndUpstreamGone(t *testing.T) {
	dir := newTestRepo(t)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", ".")
	gitIn(t, dir, "remote", "add", "origin", remote)
	gitIn(t, dir, "push", "-q", "-u", "origin", "main")

	gitIn(t, dir, "checkout", "-q", "-b", "pushed")
	commitFile(t, dir, "p.txt", "1")
	gitIn(t, dir, "push", "-q", "-u", "origin", "pushed")
	gitIn(t, dir, "push", "-q", "origin", "--delete", "pushed")
	gitIn(t, dir, "fetch", "-q", "--prune")

	r, _ := Open(dir)
	bs, _ := r.Branches("main")
	for _, b := range bs {
		if b.Name == "pushed" {
			if !b.UpstreamGone {
				t.Error("удалённый upstream не распознан")
			}
			if !b.Current {
				t.Error("pushed — текущая ветка, флаг не выставлен")
			}
		}
	}
}

func TestAheadBehindErrorMeansNotMerged(t *testing.T) {
	dir := newTestRepo(t)
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	commitFile(t, dir, "f.txt", "1")
	gitIn(t, dir, "checkout", "-q", "main")

	r, _ := Open(dir)
	if _, _, err := r.aheadBehind("no-such-base", "feature"); err == nil {
		t.Fatal("ждали ошибку на несуществующей базе")
	}

	bs, err := r.Branches("no-such-base")
	if err != nil {
		t.Fatalf("Branches не должен падать целиком: %v", err)
	}
	var found bool
	for _, b := range bs {
		if b.Name == "feature" {
			found = true
			if b.Merged {
				t.Error("при сбое подсчёта ветка не должна считаться вмёрженной")
			}
			if b.Ahead == 0 {
				t.Error("при сбое подсчёта ветка должна выглядеть имеющей уникальные коммиты")
			}
		}
	}
	if !found {
		t.Error("ветка пропала из выдачи при сбое подсчёта")
	}
}

// TestBranchesCreatedIsFirstUniqueCommit: git не хранит дату создания ветки, поэтому
// Created берётся с первого коммита, уникального для ветки — не с последнего.
func TestBranchesCreatedIsFirstUniqueCommit(t *testing.T) {
	dir := newTestRepo(t)
	first := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	second := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	commitFileAt(t, dir, "a.txt", "1", first)
	commitFileAt(t, dir, "b.txt", "2", second)
	gitIn(t, dir, "checkout", "-q", "main")

	r, _ := Open(dir)
	bs, err := r.Branches("main")
	if err != nil {
		t.Fatal(err)
	}
	var got time.Time
	var found bool
	for _, b := range bs {
		if b.Name == "feature" {
			got, found = b.Created, true
		}
	}
	if !found {
		t.Fatal("ветка feature не найдена")
	}
	if !got.Equal(first) {
		t.Fatalf("Created = %v, ждали дату первого уникального коммита %v", got, first)
	}
}

// TestBranchesCreatedFallsBackToTipWhenFullyMerged: после обычного --no-ff мержа
// main..branch пуст (коммит ветки уже предок main через мерж) — Created должен
// откатиться на дату верхушки ветки, а не остаться нулевым временем.
func TestBranchesCreatedFallsBackToTipWhenFullyMerged(t *testing.T) {
	dir := newTestRepo(t)
	tipTime := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	gitIn(t, dir, "checkout", "-q", "-b", "plain")
	commitFileAt(t, dir, "p.txt", "1", tipTime)
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "-m", "merge plain", "plain")

	r, _ := Open(dir)
	bs, err := r.Branches("main")
	if err != nil {
		t.Fatal(err)
	}
	var got time.Time
	var found bool
	for _, b := range bs {
		if b.Name == "plain" {
			got, found = b.Created, true
		}
	}
	if !found {
		t.Fatal("ветка plain не найдена")
	}
	if got.IsZero() {
		t.Fatal("Created нулевой — fallback на верхушку не сработал")
	}
	if !got.Equal(tipTime) {
		t.Fatalf("Created = %v, ждали дату верхушки %v", got, tipTime)
	}
}

func TestDeleteAndRestore(t *testing.T) {
	dir := newTestRepo(t)
	gitIn(t, dir, "checkout", "-q", "-b", "doomed")
	commitFile(t, dir, "d.txt", "1")
	head := gitIn(t, dir, "rev-parse", "doomed")
	gitIn(t, dir, "checkout", "-q", "main")

	r, _ := Open(dir)
	if err := r.Delete("doomed", true); err != nil {
		t.Fatalf("удаление: %v", err)
	}
	if bs, _ := r.Branches("main"); len(bs) != 0 {
		t.Fatalf("ветка не удалена: %+v", bs)
	}
	if err := r.Restore("doomed", head); err != nil {
		t.Fatalf("восстановление: %v", err)
	}
	bs, _ := r.Branches("main")
	if len(bs) != 1 || bs[0].Name != "doomed" {
		t.Fatal("ветка не восстановлена")
	}
	if bs[0].Head != head {
		t.Fatalf("ветка восстановлена на %s, ждали %s", bs[0].Head, head)
	}
}
