.PHONY: run demo build app test check preview clean

run:
	CGO_ENABLED=0 go run .

demo:
	CGO_ENABLED=0 go run . -demo

build:
	CGO_ENABLED=0 go build -trimpath -o bin/pixeluxe .

app: build
	mkdir -p bin/Pixeluxe.app/Contents/MacOS
	cp bin/pixeluxe bin/Pixeluxe.app/Contents/MacOS/pixeluxe
	cp packaging/Info.plist bin/Pixeluxe.app/Contents/Info.plist

test:
	CGO_ENABLED=0 go test ./...

check: test
	CGO_ENABLED=0 go vet ./...

preview:
	CGO_ENABLED=0 go run . -demo -preview docs/pixeluxe.png

clean:
	rm -rf bin
