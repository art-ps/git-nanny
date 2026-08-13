package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/art-ps/git-nanny/internal/classify"
	"github.com/art-ps/git-nanny/internal/gitrepo"
	"github.com/art-ps/git-nanny/internal/journal"
	"github.com/art-ps/git-nanny/internal/ui"
)

type Options struct {
	Merged        bool
	AllButDefault bool
	DryRun        bool
	Yes           bool
	Force         bool
	StaleDays     int
	Protect       []string
	DefaultBranch string // побеждает автоопределение, если задан
}

const noDefaultBranchMsg = "не удалось определить основную ветку — " +
	"задай её явно: git config nanny.defaultBranch <имя> или git nanny --default-branch <имя>"

// resolveDefaultBranch выбирает основную ветку: явный флаг побеждает автоопределение.
func resolveDefaultBranch(repo *gitrepo.Repo, o Options) string {
	if o.DefaultBranch != "" {
		return o.DefaultBranch
	}
	return repo.DefaultBranch()
}

// Plan — что снесём в неинтерактивном режиме. Защита и правило уникальных коммитов
// живут в classify.Entry.Deletable, здесь только выбор набора.
func Plan(entries []classify.Entry, o Options) []classify.Entry {
	// без явно выбранной области ничего не удаляем: голый запуск не должен
	// вести себя как --all-but-default
	if !o.Merged && !o.AllButDefault {
		return nil
	}
	var out []classify.Entry
	for _, e := range entries {
		if !e.Deletable(o.Force) {
			continue
		}
		if o.Merged && e.Category != classify.Merged {
			continue
		}
		out = append(out, e)
	}
	return out
}

func plural(n int, one, few, many string) string {
	switch {
	case n%10 == 1 && n%100 != 11:
		return one
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20):
		return few
	default:
		return many
	}
}

func FormatEntry(e classify.Entry, now time.Time) string {
	days := int(now.Sub(e.LastCommit).Hours() / 24)
	age := fmt.Sprintf("%d %s назад", days, plural(days, "день", "дня", "дней"))
	parts := []string{e.Name, age, fmt.Sprintf("+%d/−%d", e.Ahead, e.Behind), e.Category.String()}
	if e.Protected {
		parts = append(parts, "защищена: "+e.ProtectReason)
	}
	return strings.Join(parts, " · ")
}

func Run(dir string, o Options, out io.Writer) (int, error) {
	repo, err := gitrepo.Open(dir)
	if err != nil {
		return 1, err
	}
	def := resolveDefaultBranch(repo, o)
	if def == "" {
		fmt.Fprintln(out, noDefaultBranchMsg)
		return 1, nil
	}
	branches, err := repo.Branches(def)
	if err != nil {
		return 1, err
	}
	now := time.Now()
	protect := append(append([]string{}, repo.ProtectPatterns()...), o.Protect...)
	entries := classify.Build(branches, def, protect, now, o.StaleDays)

	if len(entries) == 0 {
		fmt.Fprintf(out, "кроме %s веток нет — убирать нечего\n", def)
		return 0, nil
	}

	plan := Plan(entries, o)
	if len(plan) == 0 {
		if !o.Merged && !o.AllButDefault {
			fmt.Fprintf(out, "веток: %d\n\n", len(entries))
			for _, e := range entries {
				fmt.Fprintln(out, "  "+FormatEntry(e, now))
			}
			fmt.Fprintln(out, "\nчто удалять — скажи явно: --merged или --all-but-default")
			return 0, nil
		}
		fmt.Fprintln(out, "под условия ничего не подошло")
		return 0, nil
	}

	fmt.Fprintf(out, "к удалению %d %s:\n", len(plan), plural(len(plan), "ветка", "ветки", "веток"))
	for _, e := range plan {
		fmt.Fprintln(out, "  "+FormatEntry(e, now))
	}
	if o.DryRun || !o.Yes {
		fmt.Fprintln(out, "\nничего не удалено. добавь --yes, чтобы выполнить")
		return 0, nil
	}

	var failed int
	for _, e := range plan {
		if err := journal.Append(journal.Record{
			Repo: repo.Dir(), Branch: e.Name, Head: e.Head,
			Category: e.Category.String(), At: time.Now(),
		}); err != nil {
			// без записи в журнал удаление необратимо — не удаляем
			fmt.Fprintf(out, "%s пропущена: не удалось записать журнал (%v)\n", e.Name, err)
			failed++
			continue
		}
		if err := repo.Delete(e.Name, o.Force || e.SquashMerged || e.Category != classify.Merged); err != nil {
			fmt.Fprintf(out, "не удалось удалить %s: %v\n", e.Name, err)
			failed++
			continue
		}
		fmt.Fprintf(out, "удалена %s\n", e.Name)
	}
	fmt.Fprintf(out, "\nвосстановить: git nanny restore\n")
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}

func RunInteractive(dir string, o Options, out io.Writer) (int, error) {
	repo, err := gitrepo.Open(dir)
	if err != nil {
		return 1, err
	}
	def := resolveDefaultBranch(repo, o)
	if def == "" {
		fmt.Fprintln(out, noDefaultBranchMsg)
		return 1, nil
	}
	branches, err := repo.Branches(def)
	if err != nil {
		return 1, err
	}
	now := time.Now()
	protect := append(append([]string{}, repo.ProtectPatterns()...), o.Protect...)
	entries := classify.Build(branches, def, protect, now, o.StaleDays)
	if len(entries) == 0 {
		fmt.Fprintf(out, "кроме %s веток нет — убирать нечего\n", def)
		return 0, nil
	}
	chosen, err := ui.Select(entries, now, o.Force)
	if err != nil {
		return 1, err
	}
	if len(chosen) == 0 {
		fmt.Fprintln(out, "ничего не выбрано")
		return 0, nil
	}
	var failed int
	for _, e := range chosen {
		if err := journal.Append(journal.Record{
			Repo: repo.Dir(), Branch: e.Name, Head: e.Head,
			Category: e.Category.String(), At: time.Now(),
		}); err != nil {
			// без записи в журнал удаление необратимо — не удаляем
			fmt.Fprintf(out, "%s пропущена: не удалось записать журнал (%v)\n", e.Name, err)
			failed++
			continue
		}
		if err := repo.Delete(e.Name, true); err != nil {
			fmt.Fprintf(out, "не удалось удалить %s: %v\n", e.Name, err)
			failed++
			continue
		}
		fmt.Fprintf(out, "удалена %s\n", e.Name)
	}
	fmt.Fprintln(out, "\nвосстановить: git nanny restore")
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}
