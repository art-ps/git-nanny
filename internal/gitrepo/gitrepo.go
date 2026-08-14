// Package gitrepo — тонкая обёртка над системным git. Здесь нет логики решений:
// только чтение фактов и выполнение удаления.
package gitrepo

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/art-ps/git-nanny/internal/classify"
)

type Repo struct{ dir string }

func Open(dir string) (*Repo, error) {
	r := &Repo{dir: dir}
	if out, err := r.git("rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository: %s", out)
	}
	// журнал должен находиться независимо от того, из какого подкаталога
	// репозитория запущена утилита — ключом служит корень, а не рабочий каталог
	if top, err := r.git("rev-parse", "--show-toplevel"); err == nil && top != "" {
		r.dir = top
	}
	return r, nil
}

// git запускает git и возвращает stdout. При ошибке возвращает текст stderr —
// его, в отличие от CombinedOutput, никогда не примут за разбираемое значение.
func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return strings.TrimSpace(stderr.String()), err
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBranch определяет основную ветку. Порядок важен: явная воля пользователя
// побеждает всё остальное, а текущая ветка — самый ненадёжный сигнал и годится,
// только если в репозитории вообще нет других веток-кандидатов. Если определить
// не удалось, возвращает "" — вызывающий код обязан ничего не удалять в этом случае.
func (r *Repo) DefaultBranch() string {
	if out, err := r.git("config", "--get", "nanny.defaultBranch"); err == nil && out != "" {
		return out
	}
	if out, err := r.git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if name := strings.TrimPrefix(out, "origin/"); name != "" {
			return name
		}
	}
	for _, name := range []string{"main", "master"} {
		if _, err := r.git("show-ref", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			return name
		}
	}
	out, err := r.git("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return ""
	}
	heads := strings.Split(out, "\n")
	if len(heads) == 1 && heads[0] != "" {
		return heads[0]
	}
	return ""
}

func (r *Repo) ProtectPatterns() []string {
	out, err := r.git("config", "--get-all", "nanny.protect")
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// worktreeBranches — ветки, занятые другими рабочими копиями: git их удалить не даст.
func (r *Repo) worktreeBranches() map[string]bool {
	out, err := r.git("worktree", "list", "--porcelain")
	res := map[string]bool{}
	if err != nil {
		return res
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "branch refs/heads/") {
			res[strings.TrimPrefix(line, "branch refs/heads/")] = true
		}
	}
	return res
}

func (r *Repo) Branches(defaultBranch string) ([]classify.Branch, error) {
	const sep = "\x1f"
	format := strings.Join([]string{
		"%(refname:short)", "%(objectname)", "%(committerdate:unix)",
		"%(upstream:track)", "%(HEAD)",
	}, sep)
	out, err := r.git("for-each-ref", "--format="+format, "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("could not read branches: %s", out)
	}
	inWorktree := r.worktreeBranches()
	cur, err := r.git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// не смогли определить текущую ветку — пустая строка, а не текст ошибки git
		cur = ""
	}

	var res []classify.Branch
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, sep)
		if len(f) < 5 {
			continue
		}
		name := f[0]
		if name == defaultBranch {
			continue // основную не показываем вовсе
		}
		ts, _ := strconv.ParseInt(f[2], 10, 64)
		b := classify.Branch{
			Name:         name,
			Head:         f[1],
			LastCommit:   time.Unix(ts, 0),
			UpstreamGone: strings.Contains(f[3], "gone"),
			Current:      strings.TrimSpace(f[4]) == "*" || name == cur,
			InWorktree:   inWorktree[name] && name != cur,
		}
		b.Created = r.createdAt(defaultBranch, name, b.LastCommit)
		ahead, behind, err := r.aheadBehind(defaultBranch, name)
		if err != nil {
			// не смогли посчитать — считаем ветку живой: пометить «вмёржена» здесь
			// значит предложить удалить ветку с непонятным содержимым
			b.Ahead, b.Behind, b.Merged = 1, 0, false
		} else {
			b.Ahead, b.Behind = ahead, behind
			if b.Ahead != 0 {
				b.SquashMerged = r.squashMerged(defaultBranch, name)
			}
			b.Merged = b.Ahead == 0 || b.SquashMerged
		}
		res = append(res, b)
	}
	return res, nil
}

// createdAt — дата первого коммита, уникального для ветки: git не хранит
// момент создания ветки. Пусто (ветка целиком внутри основной) — берём верхушку.
func (r *Repo) createdAt(base, name string, fallback time.Time) time.Time {
	out, err := r.git("log", "--reverse", "--format=%ct", base+".."+name)
	if err != nil || out == "" {
		return fallback
	}
	first := strings.SplitN(out, "\n", 2)[0]
	ts, err := strconv.ParseInt(strings.TrimSpace(first), 10, 64)
	if err != nil {
		return fallback
	}
	return time.Unix(ts, 0)
}

func (r *Repo) aheadBehind(base, name string) (ahead, behind int, err error) {
	out, err := r.git("rev-list", "--left-right", "--count", base+"..."+name)
	if err != nil {
		return 0, 0, err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", out)
	}
	if behind, err = strconv.Atoi(f[0]); err != nil {
		return 0, 0, err
	}
	if ahead, err = strconv.Atoi(f[1]); err != nil {
		return 0, 0, err
	}
	return ahead, behind, nil
}

// squashMerged: кладём дерево ветки поверх точки расхождения временным коммитом и
// спрашиваем git cherry, есть ли уже такой патч в основной ветке. Временный коммит
// остаётся висячим объектом и убирается обычным gc.
func (r *Repo) squashMerged(base, name string) bool {
	mergeBase, err := r.git("merge-base", base, name)
	if err != nil {
		return false
	}
	tree, err := r.git("rev-parse", name+"^{tree}")
	if err != nil {
		return false
	}
	tmp, err := r.git("commit-tree", tree, "-p", mergeBase, "-m", "_")
	if err != nil {
		return false
	}
	out, err := r.git("cherry", base, tmp)
	if err != nil || out == "" {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "-") {
			return false
		}
	}
	return true
}

func (r *Repo) Dir() string { return r.dir }

func (r *Repo) Delete(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	if out, err := r.git("branch", flag, name); err != nil {
		return fmt.Errorf("%s: %s", name, out)
	}
	return nil
}

func (r *Repo) Restore(name, head string) error {
	if out, err := r.git("branch", name, head); err != nil {
		return fmt.Errorf("%s: %s", name, out)
	}
	return nil
}
