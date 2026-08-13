package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/art-ps/git-nanny/internal/gitrepo"
	"github.com/art-ps/git-nanny/internal/journal"
)

// terminalAvailable — есть ли управляющий терминал. Проверяем ровно то, что
// открывает Bubble Tea: перенаправленный stdin бывает символьным устройством
// (/dev/null), поэтому os.ModeCharDevice тут не показатель.
func terminalAvailable() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

type multiFlag []string

func (m *multiFlag) String() string     { return "" }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func main() {
	var o Options
	var protect multiFlag
	flag.BoolVar(&o.Merged, "merged", false, "только вмёрженные ветки")
	flag.BoolVar(&o.AllButDefault, "all-but-default", false, "все ветки, кроме основной и защищённых")
	flag.BoolVar(&o.DryRun, "dry-run", false, "только показать план")
	flag.BoolVar(&o.Yes, "yes", false, "выполнить без вопросов")
	flag.BoolVar(&o.Force, "force", false, "удалять и ветки с уникальными коммитами")
	flag.IntVar(&o.StaleDays, "stale-days", 90, "сколько дней без коммитов считать заброшенностью")
	flag.StringVar(&o.DefaultBranch, "default-branch", "", "имя основной ветки (побеждает автоопределение)")
	flag.Var(&protect, "protect", "шаблон защищённых веток (можно повторять)")
	flag.Parse()
	o.Protect = protect

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch arg := flag.Arg(0); arg {
	case "":
		// без подкоманды — обычный режим
	case "restore":
		if err := runRestore(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	default:
		fmt.Fprintf(os.Stderr, "неизвестная подкоманда %q (известна только restore)\n", arg)
		os.Exit(2)
	}

	if !o.Merged && !o.AllButDefault && !o.DryRun {
		if !terminalAvailable() {
			// нет терминала — Bubble Tea не сможет отрисоваться: показываем тот же
			// список, что и обычный Run без области действия
			code, err := Run(dir, o, os.Stdout)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(code)
		}
		code, err := RunInteractive(dir, o, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(code)
	}

	code, err := Run(dir, o, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func runRestore(dir string) error {
	repo, err := gitrepo.Open(dir)
	if err != nil {
		return err
	}
	recs, err := journal.ForRepo(repo.Dir(), 20)
	if err != nil {
		return err
	}
	if len(recs) == 0 {
		fmt.Println("журнал пуст — этой утилитой здесь ничего не удалялось")
		return nil
	}
	for _, r := range recs {
		short := r.Head
		if len(short) > 7 {
			short = short[:7]
		}
		fmt.Printf("%s · %s · %s · %s\n",
			r.Branch, short, r.Category, r.At.Format(time.RFC3339))
	}
	fmt.Println("\nвернуть ветку: git branch <имя> <sha>")
	return nil
}
