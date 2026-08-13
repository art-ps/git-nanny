package journal

import (
	"testing"
	"time"
)

func TestAppendAndReadBack(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	recs := []Record{
		{Repo: "/tmp/a", Branch: "old", Head: "aaa", Category: "вмёржена", At: time.Now().Add(-time.Hour)},
		{Repo: "/tmp/b", Branch: "other", Head: "bbb", Category: "активная", At: time.Now().Add(-time.Minute)},
		{Repo: "/tmp/a", Branch: "new", Head: "ccc", Category: "давно не трогали", At: time.Now()},
	}
	for _, r := range recs {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ForRepo("/tmp/a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("получили %d записей, ждали 2", len(got))
	}
	if got[0].Branch != "new" {
		t.Errorf("первая запись %q, ждали new (новые первыми)", got[0].Branch)
	}
}

func TestForRepoOnMissingFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := ForRepo("/tmp/none", 10)
	if err != nil {
		t.Fatalf("на отсутствующем файле ошибки быть не должно: %v", err)
	}
	if len(got) != 0 {
		t.Fatal("ждали пустой список")
	}
}
