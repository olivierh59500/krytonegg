# Keep all development products inside the project directory.
export GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: run build test vet assets clean

run:
	go run .

build:
	mkdir -p bin
	go build -o bin/krytonegg .

test:
	go test ./...

vet:
	go vet ./...

assets:
	python3 tools/extract_adf.py

clean:
	rm -rf bin
