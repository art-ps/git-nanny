// Package journal хранит удалённые ветки, чтобы любое удаление можно было отменить.
package journal

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Record struct {
	Repo     string    `json:"repo"`
	Branch   string    `json:"branch"`
	Head     string    `json:"head"`
	Category string    `json:"category"`
	At       time.Time `json:"at"`
}

func Path() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "git-nanny", "deleted.jsonl")
}

func Append(rec Record) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(rec)
}

func ForRepo(repoDir string, limit int) ([]Record, error) {
	f, err := os.Open(Path())
	if os.IsNotExist(err) {
		return nil, nil // журнала ещё нет — это не ошибка
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue // битую строку пропускаем, журнал не должен ронять утилиту
		}
		if rec.Repo == repoDir {
			all = append(all, rec)
		}
	}
	// новые первыми
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, sc.Err()
}
