.PHONY: generate build vet test verify run doc doc-http clean

# templ のテンプレートから Go を生成する。*_templ.go はコミットしてあるので
# 通常のビルドでは不要。.templ を編集したときだけ実行する。
generate:
	go tool templ generate

build:
	go build -o game.exe ./cmd/game

vet:
	go vet ./...

test:
	go test -count=1 ./...

verify: generate build vet test

run:
	go run ./cmd/game -addr 127.0.0.1:8080

# ルールパッケージのドキュメントを端末に出す (Example 付き)
doc:
	go doc -all ./internal/rules
	go doc -all ./internal/rules/daifugo
	go doc -all ./internal/rules/napoleon

# ブラウザでドキュメントを見る。起動後 数秒待ってから
# http://127.0.0.1:6060/pkg/hearts/internal/rules/ を開く。
#
# godoc は Go 1.13 で本体から外れて golang.org/x/tools へ移った (非推奨だが動く)。
# @latest で取るので go.mod の依存には入らない。
doc-http:
	go run golang.org/x/tools/cmd/godoc@latest -http=127.0.0.1:6060

clean:
	rm -f game.exe
