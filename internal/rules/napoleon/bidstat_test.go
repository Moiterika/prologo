package napoleon_test

import (
	"math/rand"
	"testing"

	nrules "hearts/internal/rules/napoleon"
)

// 競りが現実的なところで落ち着くこと。
//
// CPU は合法な宣言からランダムに選ぶが、選択肢の作り方 (napoleon_ai.pl の
// bid_candidates/3) で「宣言が上がるほどパスしやすい」ように重みを付けてある。
// これが効いていないと宣言が 20 枚まで吊り上がり、ナポレオン軍がまず勝てない
// ゲームになってしまう。
func TestBiddingSettlesAtPlayableLevels(t *testing.T) {
	e, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := nrules.DefaultOptions().Apply(e); err != nil {
		t.Fatal(err)
	}
	deck, err := e.Deck()
	if err != nil {
		t.Fatal(err)
	}

	const trials = 120
	rng := rand.New(rand.NewSource(1))
	high, redeals := 0, 0
	for trial := 0; trial < trials; trial++ {
		bid := runBidding(t, e, deck, rng)
		switch {
		case bid == nil:
			redeals++
		case bid.Count >= 16:
			high++
		}
	}
	// 1 割を超えて 16 枚以上の宣言になるようなら、重み付けが壊れている。
	if high*10 > trials {
		t.Errorf("16 枚以上の宣言が %d / %d 回もあります", high, trials)
	}
	// 流局ばかりでも遊べない。
	if redeals*4 > trials {
		t.Errorf("誰も宣言しない回が %d / %d 回もあります", redeals, trials)
	}
}

// runBidding は 5 人の競りを 1 回まわし、決まった宣言を返す。
// 誰も宣言しなければ nil。
func runBidding(t *testing.T, e *nrules.Engine, deck []nrules.Card, rng *rand.Rand) *nrules.Bid {
	t.Helper()
	d := append([]nrules.Card(nil), deck...)
	rng.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	hands := [][]nrules.Card{d[0:10], d[10:20], d[20:30], d[30:40], d[40:50]}

	var current *nrules.Bid
	passed := make([]bool, 5)
	turn := 0
	for step := 0; step < 500; step++ {
		active := 0
		for _, p := range passed {
			if !p {
				active++
			}
		}
		if active == 0 || (current != nil && active <= 1) {
			break
		}
		if !passed[turn] {
			bid, pass, err := e.ChooseBid(hands[turn], current, rng.Intn(1<<30))
			if err != nil {
				t.Fatal(err)
			}
			if pass {
				passed[turn] = true
			} else {
				ok, err := e.BidOK(bid, current)
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					t.Fatalf("CPU が通らない宣言を選びました: %s", bid.Label())
				}
				current = &nrules.Bid{Count: bid.Count, Suit: bid.Suit}
			}
		}
		turn = (turn + 1) % 5
	}
	return current
}
