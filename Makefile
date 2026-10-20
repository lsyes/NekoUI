.PHONY: run gallery ime test race bench vet fmt screenshot

run:
	go run ./examples/showcase

gallery:
	go run ./examples/gallery

ime:
	go run ./examples/ime -backend x11

test:
	go test ./...

race:
	go test -race ./...

bench:
	go test ./internal/ui -run '^$$' -bench . -benchmem

vet:
	go vet ./...

fmt:
	gofmt -w .

screenshot:
	go run ./examples/showcase -screenshot docs/assets/showcase.png
	go run ./examples/gallery -screenshot docs/assets/gallery.png
