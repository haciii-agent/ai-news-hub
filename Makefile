.PHONY: build run test clean docker-build docker-up docker-down

BINARY := ai-news-hub
PORT   ?= 8080

# ---------- Go ----------
build:
	CGO_ENABLED=1 go build -o $(BINARY) .

run: build
	./$(BINARY)

run-dev:
	CGO_ENABLED=1 go run .

test:
	CGO_ENABLED=1 go test ./... -v -timeout 30s

clean:
	rm -f $(BINARY)
	rm -rf data/

# ---------- Docker ----------
docker-build:
	docker build -t ai-news-hub:latest .

docker-up: docker-build
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ---------- Helpers ----------
fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet
	@echo "lint passed ✅"
