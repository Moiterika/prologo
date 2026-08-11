package web

// viewmodel_test.go — 画面用構造体の組み立ての検証。

import (
	"testing"

	"hearts/internal/game"
	"hearts/internal/rules"
)

// 円卓は「あなた」が手前 (bottom)、そこから席順に左回り。
func TestSeatAreas(t *testing.T) {
	cases := map[int][]string{
		3: {"bottom", "left", "right"},
		4: {"bottom", "left", "top", "right"},
		5: {"bottom", "left", "topleft", "topright", "right"},
	}
	for players, want := range cases {
		got := seatAreas(players)
		if len(got) != len(want) {
			t.Errorf("%d人: seatAreas = %v; want %v", players, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%d人: seatAreas = %v; want %v", players, got, want)
				break
			}
		}
	}
}

// 出したカードは、出した席にだけ付く。
func TestSeatsCarryPlayedCards(t *testing.T) {
	g := newTestGame(t, 20260811, 100)

	// 交換を済ませてトリックプレイに入る
	for g.Phase() == game.PhasePassing {
		autoStep(t, g)
	}
	// 人間が先手だと、この時点ではまだ場が空。1 手進めて場に札を出す。
	if len(g.Played()) == 0 {
		autoStep(t, g)
	}

	vm, err := buildTableVM(g, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(vm.Seats) != g.Players() {
		t.Fatalf("席数 = %d; want %d", len(vm.Seats), g.Players())
	}
	if vm.Seats[game.HumanSeat].Area != "bottom" || !vm.Seats[game.HumanSeat].IsYou {
		t.Errorf("あなたの席 = %+v; 手前 (bottom) のはず", vm.Seats[game.HumanSeat])
	}

	// 実際に場に出ているカードと突き合わせる
	played := map[int]rules.Card{}
	order := map[int]int{}
	for i, pi := range g.PlayedBy() {
		played[pi] = g.Played()[i]
		order[pi] = i + 1
	}

	if len(played) == 0 {
		t.Fatal("誰もカードを出していません")
	}

	for i, s := range vm.Seats {
		card, ok := played[i]
		if !ok {
			if s.Played != nil {
				t.Errorf("%s はまだ出していないのにカードが付いています: %+v", s.Name, s.Played)
			}
			continue
		}
		if s.Played == nil {
			t.Errorf("%s が出した %v が席に付いていません", s.Name, card)
			continue
		}
		if s.Played.Suit != rules.SuitSymbol(card.Suit) || s.Played.Rank != rules.RankLabel(card.Rank) {
			t.Errorf("%s の席のカード = %s%s; want %s",
				s.Name, s.Played.Suit, s.Played.Rank, rules.CardLabel(card))
		}
		if s.PlayOrder != order[i] {
			t.Errorf("%s の出した順 = %d; want %d", s.Name, s.PlayOrder, order[i])
		}
	}
}

// トリックが揃うまで「取り」の印は付かない。
func TestWinnerMarkOnlyAfterTrick(t *testing.T) {
	g := newTestGame(t, 5, 100)
	for g.Phase() == game.PhasePassing {
		autoStep(t, g)
	}

	// まだ揃っていない段階では誰にも印が無い
	if g.Phase() == game.PhasePlaying {
		vm, err := buildTableVM(g, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range vm.Seats {
			if s.Winner {
				t.Errorf("トリックの途中なのに %s に「取り」が付いています", s.Name)
			}
		}
	}

	// トリックが揃うまで進める
	for step := 0; g.Phase() != game.PhaseTrickDone && step < 50; step++ {
		autoStep(t, g)
	}
	if g.Phase() != game.PhaseTrickDone {
		t.Fatal("トリックが終わりません")
	}

	vm, err := buildTableVM(g, "")
	if err != nil {
		t.Fatal(err)
	}
	winners := 0
	for i, s := range vm.Seats {
		if s.Winner {
			winners++
			if i != g.TrickWinner() {
				t.Errorf("%s に「取り」が付いていますが勝者は %s です",
					s.Name, g.Names()[g.TrickWinner()])
			}
		}
	}
	if winners != 1 {
		t.Errorf("「取り」の印が %d 個; ちょうど 1 個のはず", winners)
	}
	if vm.CenterText == "" {
		t.Error("中央の表示が空です")
	}
}
