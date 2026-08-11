package main

// ハート (Hearts) — CPU 3人と対戦するブラウザゲーム。
//
// ゲームのルール判定は Prolog で書かれた hearts.pl とその上に重ねた
// hearts_rules.pl / hearts_ai.pl が担当する。Go は純Go実装の Prolog 処理系
// (github.com/ichiban/prolog) にそれらを読み込ませ、画面を templ で描いて
// HTTP で返すだけ。
//
//   compat.pl       … SWI 固有述語の互換レイヤ (hearts.pl を無修正で読むため)
//   hearts.pl       … 元のルール定義 (変更なし)
//   hearts_rules.pl … ハートブレイク・第1トリック・パス・精算などの追加規則
//   hearts_ai.pl    … CPU の思考

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "待ち受けアドレス")
	limit := flag.Int("limit", 100, "この失点に達したらゲーム終了")
	players := flag.Int("players", 0, "人数 3/4/5 (0 なら hearts.pl の既定値である4人)")
	seed := flag.Int64("seed", 0, "乱数シード (0 なら毎回変わる)")
	plDir := flag.String("pl", "", "このディレクトリの .pl を読み込む (既定は実行ファイルへの埋め込み)")
	flag.Parse()

	// 起動時に一度 Prolog を読んで、ルールファイルの誤りをここで弾く。
	srv := NewServer(*plDir, *players, *limit, *seed)
	rules, err := srv.newRules()
	if err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
	if _, err := srv.startGame(rules, DefaultOptions()); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	srv.Routes(mux)

	log.Printf("ハートを開始します: http://%s/", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}
