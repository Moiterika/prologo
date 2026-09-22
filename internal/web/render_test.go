package web

// render_test.go — 対局を最後まで進めてから、各局面の画面を実際に描いてみる。
//
// 状態機械そのものは internal/game 以下のテストが見ているので、ここでは
// 「ビューモデルへの詰め替えとテンプレートが通ること」だけを確かめる。
// 結果画面は 1 ゲーム遊びきらないと出てこないので、HTTP 越しに操作する
// テストでは届きにくい。

import (
	"bytes"
	"context"
	"math/rand"
	"strings"
	"testing"

	"hearts/internal/cards"
	dgame "hearts/internal/game/daifugo"
	ngame "hearts/internal/game/napoleon"
	drules "hearts/internal/rules/daifugo"
	nrules "hearts/internal/rules/napoleon"
	"hearts/internal/web/view"
)

// =============================================
// 大富豪
// =============================================

func TestDaifugoRendersEveryPhase(t *testing.T) {
	// カード交換や 10捨ての画面は、称号や配札次第でしか出てこない。
	// 全ての局面が揃うまで種を変えて遊び直す。
	all := []string{
		"playing", "exchange", "jack_back", "ten_discard", "seven_give",
		"round_end", "game_over",
	}
	seen := map[string]bool{}
	for seed := int64(1); seed <= 12 && !hasAll(seen, all...); seed++ {
		engine, err := drules.New("")
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewSource(seed))
		opts := drules.DefaultOptions()
		opts.FirstPlay = "diamond3_free" // ♦3 縛りを外して自動操作を単純にする
		opts.TenDiscard = "on"
		opts.SevenGive = "on"
		opts.JackBack = "all"
		opts.JackBackMode = "optional"
		g, err := dgame.New(engine, rng, 2, opts)
		if err != nil {
			t.Fatal(err)
		}

		for step := 0; step < 3000; step++ {
			phase := string(g.Phase())
			if !seen[phase] {
				seen[phase] = true
				renderDaifugoPage(t, g, phase)
			}
			if g.Phase() == dgame.PhaseGameOver {
				break
			}
			advanceDaifugo(t, g, engine, rng)
		}
	}

	for _, want := range all {
		if !seen[want] {
			t.Errorf("局面 %s を一度も描いていません (到達した局面: %v)", want, keys(seen))
		}
	}
}

func hasAll(seen map[string]bool, phases ...string) bool {
	for _, p := range phases {
		if !seen[p] {
			return false
		}
	}
	return true
}

func renderDaifugoPage(t *testing.T, g *dgame.Game, phase string) {
	t.Helper()
	vm, err := buildDaifugoTableVM(g, "")
	if err != nil {
		t.Fatalf("%s のビューモデルを作れません: %v", phase, err)
	}
	var buf bytes.Buffer
	if err := view.DaifugoTablePage(vm).Render(context.Background(), &buf); err != nil {
		t.Fatalf("%s の描画に失敗しました: %v", phase, err)
	}
	html := buf.String()
	if !strings.Contains(html, "大富豪") {
		t.Errorf("%s の画面にゲーム名がありません", phase)
	}

	switch phase {
	case "round_end":
		if !strings.Contains(html, "ゲームの結果") {
			t.Errorf("結果画面になっていません")
		}
		// 称号が日本語で出ていること
		if !strings.Contains(html, "大富豪") || !strings.Contains(html, "大貧民") {
			t.Errorf("称号が出ていません")
		}
		if len(vm.Ranking) != g.Players() {
			t.Errorf("順位表が %d 行しかありません", len(vm.Ranking))
		}
	case "game_over":
		if !strings.Contains(html, "勝ちです") {
			t.Errorf("最終結果になっていません")
		}
	case "exchange":
		if !strings.Contains(html, "渡すカードを") {
			t.Errorf("交換画面になっていません")
		}
	case "jack_back":
		if !strings.Contains(html, "Jバックを使いますか") {
			t.Errorf("Jバックの選択画面になっていません")
		}
		if len(vm.JackBackCards) == 0 {
			t.Errorf("出した手が画面に出ていません")
		}
	case "ten_discard":
		if !strings.Contains(html, "10捨て") || !strings.Contains(html, "捨てるカードを") {
			t.Errorf("10捨ての画面になっていません")
		}
		if vm.HandOverNeed <= 0 {
			t.Errorf("捨てる枚数が %d です", vm.HandOverNeed)
		}
	case "seven_give":
		if !strings.Contains(html, "7渡し") || !strings.Contains(html, "渡すカードを") {
			t.Errorf("7渡しの画面になっていません")
		}
		if vm.HandOverToName == "" {
			t.Errorf("渡す相手が出ていません")
		}
	}
}

// advanceDaifugo は人間の席を 1 手だけランダムに進める。
func advanceDaifugo(t *testing.T, g *dgame.Game, engine *drules.Engine, rng *rand.Rand) {
	t.Helper()
	switch g.Phase() {
	case dgame.PhaseRoundEnd:
		if err := g.Next(); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseExchange:
		if err := g.SubmitExchange(firstNumbers(g.ExchangeNeed())); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseJackBack:
		if err := g.SubmitJackBack(true); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseTenDiscard:
		if err := g.SubmitTenDiscard(firstNumbers(g.HandOverNeed())); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseSevenGive:
		if err := g.SubmitSevenGive(firstNumbers(g.HandOverNeed())); err != nil {
			t.Fatal(err)
		}
	case dgame.PhasePlaying:
		plays, err := engine.LegalPlays(g.Hand(), g.Field(), g.Reversed())
		if err != nil {
			t.Fatal(err)
		}
		if len(plays) == 0 {
			if err := g.Pass(); err != nil {
				t.Fatal(err)
			}
			return
		}
		play := plays[rng.Intn(len(plays))]
		hand := g.Hand()
		nums := make([]int, 0, len(play))
		used := map[int]bool{}
		for _, c := range play {
			for i, h := range hand {
				if h == c && !used[i] {
					used[i] = true
					nums = append(nums, i+1)
					break
				}
			}
		}
		if err := g.PlayCards(nums); err != nil {
			t.Fatal(err)
		}
	}
}

// =============================================
// ナポレオン
// =============================================

func TestNapoleonRendersEveryPhase(t *testing.T) {
	engine, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(4))
	g, err := ngame.New(engine, rng, 2, nrules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for step := 0; step < 3000; step++ {
		phase := string(g.Phase())
		if !seen[phase] {
			seen[phase] = true
			renderNapoleonPage(t, g, phase)
		}
		if g.Phase() == ngame.PhaseGameOver {
			break
		}
		advanceNapoleon(t, g, rng)
	}

	for _, want := range []string{"playing", "trick_done", "round_done", "game_over"} {
		if !seen[want] {
			t.Errorf("局面 %s を一度も描いていません (到達した局面: %v)", want, keys(seen))
		}
	}
}

func renderNapoleonPage(t *testing.T, g *ngame.Game, phase string) {
	t.Helper()
	vm, err := buildNapoleonTableVM(g, "")
	if err != nil {
		t.Fatalf("%s のビューモデルを作れません: %v", phase, err)
	}
	var buf bytes.Buffer
	if err := view.NapoleonTablePage(vm).Render(context.Background(), &buf); err != nil {
		t.Fatalf("%s の描画に失敗しました: %v", phase, err)
	}
	html := buf.String()

	switch phase {
	case "bidding":
		if !strings.Contains(html, "ナポレオン宣言") {
			t.Errorf("競りの画面になっていません")
		}
	case "playing":
		// 指名したカードと、取った絵札の枚数はプレイ中ずっと見えている
		if vm.AideCardLabel == "" {
			t.Errorf("指名したカードがビューモデルに入っていません")
		}
		if !strings.Contains(html, "副官指名") {
			t.Errorf("副官の指名カードが画面に出ていません")
		}
		if !strings.Contains(html, "絵札") {
			t.Errorf("取った絵札の枚数が画面に出ていません")
		}
	case "round_done":
		if !strings.Contains(html, "ゲームの結果") {
			t.Errorf("精算の画面になっていません")
		}
		if !strings.Contains(html, "ナポレオン軍の勝ち") && !strings.Contains(html, "連合軍の勝ち") {
			t.Errorf("勝敗が出ていません")
		}
		// 精算では副官が明かされる
		if !g.Solo() && vm.AideName == "" {
			t.Errorf("副官が明かされていません")
		}
	case "game_over":
		if !strings.Contains(html, "勝ちです") {
			t.Errorf("最終結果になっていません")
		}
	}
}

func advanceNapoleon(t *testing.T, g *ngame.Game, rng *rand.Rand) {
	t.Helper()
	switch g.Phase() {
	case ngame.PhaseBidding:
		// 人間は必ずパスする。CPU だけで競りが決まる。
		if err := g.SubmitBid(0, "", true); err != nil {
			t.Fatal(err)
		}
	case ngame.PhaseNaming:
		choices := g.AideChoices()
		c := choices[rng.Intn(len(choices))]
		if err := g.SubmitAide(c.Suit, c.Rank); err != nil {
			t.Fatal(err)
		}
	case ngame.PhaseDiscarding:
		nums := make([]int, 0, g.WidowSize())
		for i := 1; i <= g.WidowSize(); i++ {
			nums = append(nums, i)
		}
		if err := g.SubmitDiscard(nums); err != nil {
			t.Fatal(err)
		}
	case ngame.PhasePlaying:
		legal, err := g.LegalCards()
		if err != nil {
			t.Fatal(err)
		}
		pick := legal[rng.Intn(len(legal))]
		n := cards.IndexOf(g.Hand(), pick)
		if err := g.PlayCard(n + 1); err != nil {
			t.Fatal(err)
		}
	case ngame.PhaseTrickDone, ngame.PhaseRoundDone:
		if err := g.Next(); err != nil {
			t.Fatal(err)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// firstNumbers は手札の先頭から n 枚ぶんの表示番号。
func firstNumbers(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}
