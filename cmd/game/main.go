// Command game はハート・大富豪・ナポレオンを CPU と対戦できるブラウザゲーム。
//
// ゲームのルール判定はすべて Prolog で書かれている (internal/rules 以下)。
// Go は純 Go 実装の Prolog 処理系にルールファイルを読み込ませ、進行を
// 管理し (internal/game 以下)、画面を templ で描いて HTTP で返す
// (internal/web)。
//
//	/           ハート
//	/daifugo    大富豪
//	/napoleon   ナポレオン
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
	limit := flag.Int("limit", 100, "ハート: この失点に達したらゲーム終了")
	players := flag.Int("players", 0, "ハート: 人数 3/4/5 (0 なら hearts.pl の既定値である4人)")
	games := flag.Int("games", 5, "大富豪・ナポレオン: 通して遊ぶゲーム数")
	seed := flag.Int64("seed", 0, "乱数シード (0 なら毎回変わる)")
	plDir := flag.String("pl", "", "この下の .pl を読み込む (既定は実行ファイルへの埋め込み)")
	flag.Parse()

	cfg := web.Config{
		PLDir:   *plDir,
		Players: *players,
		Limit:   *limit,
		Games:   *games,
		Seed:    *seed,
	}
	if err := run(*addr, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}

func run(addr string, cfg web.Config) error {
	srv := web.NewServer(cfg)

	// 起動時に一度 Prolog を読んでゲームを組み立て、ルールファイルの誤りを
	// ここで弾く。実際の対局はブラウザからの接続ごとに作り直す。
	engine, err := rules.New(cfg.PLDir)
	if err != nil {
		return err
	}
	if cfg.Players != 0 {
		if err := engine.SetPlayers(cfg.Players); err != nil {
			return err
		}
	}
	if _, err := srv.StartGame(engine, rules.DefaultOptions()); err != nil {
		return err
	}

	mux := http.NewServeMux()
	srv.Routes(mux)

	log.Printf("ゲームを開始します: http://%s/", addr)
	log.Printf("  ハート      http://%s/", addr)
	log.Printf("  大富豪      http://%s/daifugo", addr)
	log.Printf("  ナポレオン  http://%s/napoleon", addr)
	return http.ListenAndServe(addr, mux)
}
