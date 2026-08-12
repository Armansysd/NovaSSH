# NovaSSH Enterprise PRO Makefile

BINARY_NAME=novassh
VERSION=3.0-Enterprise-Desktop

all: build-linux build-windows

build-linux:
	@echo "--- Building Linux x86_64 binary ---"
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$(VERSION)" -o novassh-linux-amd64 ./main.go

build-windows:
	@echo "--- Building Windows x86_64 (.exe) binary without console window (-H=windowsgui) ---"
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui -X main.Version=$(VERSION)" -o novassh-win-amd64.exe ./main.go

run:
	@echo "--- Running NovaSSH Enterprise Server on :8080 ---"
	./novassh-linux-amd64 -port=8080

clean:
	rm -rf bin/ novassh-linux-amd64 novassh-win-amd64.exe
