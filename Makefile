BIN := git-nanny

build:
	go build -o $(BIN) ./cmd/git-nanny

test:
	go test ./...

install:
	go install ./cmd/git-nanny

.PHONY: build test install
