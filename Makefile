build:
	go build -o validation_bot cmd/validation_bot.go

run:
	go run cmd/validation_bot.go

clean:
	go clean
	rm -f validation_bot

test:
	go test -p 4 -v ./...

.PHONY: build run clean test
