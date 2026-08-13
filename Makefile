BIN := git-nanny
PREFIX ?= /usr/local
MANDIR ?= $(PREFIX)/share/man/man1

build:
	go build -o $(BIN) ./cmd/git-nanny

test:
	go test ./...

install:
	go install ./cmd/git-nanny

# Ставит man-страницу отдельно: go install кладёт только бинарь.
install-man:
	install -d $(MANDIR)
	install -m 0644 man/$(BIN).1 $(MANDIR)/$(BIN).1

.PHONY: build test install install-man
