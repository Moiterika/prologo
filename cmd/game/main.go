// Command game はハート (Hearts) を CPU 3 人と対戦できるブラウザゲーム。
//
// ゲームのルール判定は Prolog で書かれた hearts.pl とその上に重ねた
// hearts_rules.pl / hearts_ai.pl が担当する (internal/rules)。Go は純 Go 実装の
// Prolog 処理系にそれらを読み込ませ、進行を管理し (internal/game)、
// 画面を templ で描いて HTTP で返す (internal/web)。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"hearts/internal/rules"
	"hearts/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "待ち受けアドレス")
	limit := flag.Int("limit", 100, "この失点に達したらゲーム終了")
	players := flag.Int("players", 0, "人数 3/4/5 (0 なら hearts.pl の既定値である4人)")
	seed := flag.Int64("seed", 0, "乱数シード (0 なら毎回変わる)")
	plDir := flag.String("pl", "", "このディレクトリの .pl を読み込む (既定は実行ファイルへの埋め込み)")
	flag.Parse()

	if err := run(*addr, *limit, *players, *seed, *plDir); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}

func run(addr string, limit, players int, seed int64, plDir string) error {
	srv := web.NewServer(plDir, players, limit, seed)

	// 起動時に一度 Prolog を読んでゲームを組み立て、ルールファイルの誤りを
	// ここで弾く。実際の対局はブラウザからの接続ごとに作り直す。
	engine, err := rules.New(plDir)
	if err != nil {
		return err
	}
	if players != 0 {
		if err := engine.SetPlayers(players); err != nil {
			return err
		}
	}
	if _, err := srv.StartGame(engine, rules.DefaultOptions()); err != nil {
		return err
	}

	mux := http.NewServeMux()
	srv.Routes(mux)

	log.Printf("ハートを開始します: http://%s/", addr)
	return http.ListenAndServe(addr, mux)
}
