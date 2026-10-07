# Keep all development products inside the project directory.
export GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: run build test vet assets assets-ready android android-run touch clean validate-progression

run: assets-ready
	go run .

build: assets-ready
	mkdir -p bin
	go build -o bin/krytonegg .

test: assets-ready
	go test ./...

vet: assets-ready
	go vet ./...

assets:
	python3 tools/prepare_assets.py --force

assets-ready:
	python3 tools/prepare_assets.py

android:
	./scripts/build-android.sh

android-run:
	./scripts/run-android.sh

touch: assets-ready
	go run . -touch

validate-progression: assets-ready
	mkdir -p captures
	go run ./cmd/validate-progression -mode campaign -seed 42 -speed 12 -reaction 4 -out captures/progression.json

clean:
	rm -rf bin
