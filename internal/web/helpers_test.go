package web

// helpers_test.go — web パッケージのテストが使う小さな道具。
//
// game パッケージのテスト用ヘルパーは他のパッケージからは見えないので、
// 画面の組み立てを直接確かめたいときに使う分だけここに置く。

import (
	"math/rand"
	"testing"

	"hearts/internal/game"
	"hearts/internal/rules"
)

func newTestGame(t *testing.T, seed int64, limit int) *game.Game {
	t.Helper()
	engine, err := rules.New("")
	if err != nil {
		t.Fatalf("Prolog の読み込みに失敗: %v", err)
	}
	g, err := game.New(engine, rand.New(rand.NewSource(seed)), limit, rules.DefaultOptions())
	if err != nil {
		t.Fatalf("ゲームを開始できません: %v", err)
	}
	return g
}

// autoStep は人間の席を「合法手の先頭を選ぶ」だけで 1 手進める。
func autoStep(t *testing.T, g *game.Game) {
	t.Helper()
	switch g.Phase() {
	case game.PhasePassing:
		nums := make([]int, g.PassCount())
		for i := range nums {
			nums[i] = i + 1
		}
		if err := g.SubmitPass(nums); err != nil {
			t.Fatalf("カード交換に失敗: %v", err)
		}
	case game.PhasePlaying:
		legal, err := g.LegalCards()
		if err != nil {
			t.Fatalf("合法手の取得に失敗: %v", err)
		}
		if len(legal) == 0 {
			t.Fatal("あなたの手番なのに出せるカードがありません")
		}
		if err := g.PlayCard(handNumber(t, g, legal[0])); err != nil {
			t.Fatalf("カードを出せません: %v", err)
		}
	case game.PhaseTrickDone, game.PhaseRoundDone:
		if err := g.Next(); err != nil {
			t.Fatalf("進行に失敗: %v", err)
		}
	}
}

// handNumber は表示順のどこにそのカードがあるかを返す (1 始まり)。
func handNumber(t *testing.T, g *game.Game, card rules.Card) int {
	t.Helper()
	for i, c := range g.Hand() {
		if c == card {
			return i + 1
		}
	}
	t.Fatalf("%v が手札にありません", card)
	return 0
}
