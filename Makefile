.PHONY: generate build vet test verify run clean

# templ のテンプレートから Go を生成する。*_templ.go はコミットしてあるので
# 通常のビルドでは不要。.templ を編集したときだけ実行する。
generate:
	go tool templ generate

build:
	go build -o hearts.exe .

vet:
	go vet ./...

test:
	go test -count=1 ./...

verify: generate build vet test

run:
	go run . -addr 127.0.0.1:8080

clean:
	rm -f hearts.exe
