package gitrepo

import (
	"testing"
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
