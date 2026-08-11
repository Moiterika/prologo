package main

// ハーツ (Hearts) — CPU 3人と対戦する CUI ゲーム。
//
// ゲームのルール判定は Prolog で書かれた hearts.pl とその上に重ねた
// hearts_rules.pl / hearts_ai.pl が担当し、Go は純Go実装の Prolog 処理系
// (github.com/ichiban/prolog) にそれらを読み込ませて対話を回すだけ。
//
//   compat.pl       … SWI 固有述語の互換レイヤ (hearts.pl を無修正で読むため)
//   hearts.pl       … 元のルール定義 (変更なし)
//   hearts_rules.pl … ハートブレイク・第1トリック・パス・精算などの追加規則
//   hearts_ai.pl    … CPU の思考

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"
)

func main() {
	limit := flag.Int("limit", 100, "この失点に達したらゲーム終了")
	seed := flag.Int64("seed", 0, "乱数シード (0 なら毎回変わる)")
	players := flag.Int("players", 0, "人数 3/4/5 (0 なら hearts.pl の既定値である4人)")
	demo := flag.Bool("demo", false, "あなたの席も CPU が打つ自動再生モード")
	suitMode := flag.String("suit", suitModeAuto,
		"スートの表示: auto | letter (S/H/D/C) | symbol (♠♥♦♣ を1桁扱い) | wide (♠♥♦♣ を2桁扱い)")
	noColor := flag.Bool("nocolor", false, "色を使わない")
	plDir := flag.String("pl", "", "このディレクトリの .pl を読み込む (既定は実行ファイルへの埋め込み)")
	flag.Parse()

	vtOK := setupConsole()

	if err := run(*limit, *seed, *players, *demo, *suitMode, *noColor || !vtOK, *plDir); err != nil {
		if errors.Is(err, errQuit) {
			fmt.Println("\n中断しました。またどうぞ。")
			return
		}
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}

func run(limit int, seed int64, players int, demo bool, suitMode string, noColor bool, plDir string) error {
	letters, ambiguousWide, err := resolveSuitMode(suitMode)
	if err != nil {
		return err
	}

	rules, err := NewRules(plDir)
	if err != nil {
		return err
	}

	if players != 0 {
		if err := rules.SetPlayers(players); err != nil {
			return err
		}
	}

	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	ui := &UI{
		in:            bufio.NewReader(os.Stdin),
		out:           os.Stdout,
		rules:         rules,
		color:         !noColor,
		auto:          demo,
		letters:       letters,
		ambiguousWide: ambiguousWide,
	}

	game, err := NewGame(rules, ui, rng, limit, demo)
	if err != nil {
		return err
	}
	return game.Run()
}

// resolveSuitMode は -suit の値を (S/H/D/C を使うか, ♠♥♦♣ を2桁と数えるか) に変換する。
//
// ♠♥♦♣ は East Asian Ambiguous 幅の文字で、端末によって 1 桁にも 2 桁にも
// 描画される。Windows のコンソール (PowerShell) では実際の描画と桁数の想定が
// 食い違って表示が崩れるため、auto では文字化けしない S/H/D/C を既定にする。
func resolveSuitMode(mode string) (letters, ambiguousWide bool, err error) {
	switch mode {
	case suitModeAuto:
		return runtime.GOOS == "windows", false, nil
	case suitModeLetter:
		return true, false, nil
	case suitModeSymbol:
		return false, false, nil
	case suitModeWide:
		return false, true, nil
	default:
		return false, false, fmt.Errorf("-suit の値が不正です: %q (auto / letter / symbol / wide)", mode)
	}
}
