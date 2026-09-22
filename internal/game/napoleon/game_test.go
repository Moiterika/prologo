package napoleon_test

import (
	"math/rand"
	"testing"

	"hearts/internal/cards"
	ngame "hearts/internal/game/napoleon"
	nrules "hearts/internal/rules/napoleon"
)

// autoPlay は「人間の席」も含めて全員をランダムに動かし、1 卓を最後まで進める。
func autoPlay(t *testing.T, g *ngame.Game, engine *nrules.Engine, rng *rand.Rand) {
	t.Helper()
	for step := 0; step < 5000; step++ {
		switch g.Phase() {
		case ngame.PhaseGameOver:
			return
		case ngame.PhaseBidding:
			bids, err := g.AvailableBids()
			if err != nil {
				t.Fatal(err)
			}
			// たまに宣言し、たいていはパスする
			if len(bids) > 0 && rng.Intn(4) == 0 {
				b := bids[rng.Intn(min(len(bids), 4))]
				if err := g.SubmitBid(b.Count, b.Suit, false); err != nil {
					t.Fatalf("SubmitBid: %v", err)
				}
			} else if err := g.SubmitBid(0, "", true); err != nil {
				t.Fatalf("SubmitBid(pass): %v", err)
			}
		case ngame.PhaseNaming:
			choices := g.AideChoices()
			if len(choices) == 0 {
				t.Fatal("副官の候補がありません")
			}
			c := choices[rng.Intn(len(choices))]
			if err := g.SubmitAide(c.Suit, c.Rank); err != nil {
				t.Fatalf("SubmitAide: %v", err)
			}
		case ngame.PhaseDiscarding:
			nums := make([]int, 0, g.WidowSize())
			for i := 1; i <= g.WidowSize(); i++ {
				nums = append(nums, i)
			}
			if err := g.SubmitDiscard(nums); err != nil {
				t.Fatalf("SubmitDiscard: %v", err)
			}
		case ngame.PhasePlaying:
			legal, err := g.LegalCards()
			if err != nil {
				t.Fatal(err)
			}
			if len(legal) == 0 {
				t.Fatal("人間の手番なのに出せるカードがありません")
			}
			hand := g.Hand()
			pick := legal[rng.Intn(len(legal))]
			n := cards.IndexOf(hand, pick)
			if n < 0 {
				t.Fatalf("選んだカード %v が手札にありません", cards.Label(pick))
			}
			if err := g.PlayCard(n + 1); err != nil {
				t.Fatalf("PlayCard: %v", err)
			}
		case ngame.PhaseTrickDone, ngame.PhaseRoundDone:
			if err := g.Next(); err != nil {
				t.Fatalf("Next: %v", err)
			}
		default:
			t.Fatalf("知らない局面です: %s", g.Phase())
		}
	}
	t.Fatal("5000 手進めても終わりませんでした")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPlayThroughDefault(t *testing.T) {
	engine, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(11))
	g, err := ngame.New(engine, rng, 3, nrules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	autoPlay(t, g, engine, rng)

	if g.Phase() != ngame.PhaseGameOver {
		t.Fatalf("最後まで進みませんでした: %s", g.Phase())
	}
	// 『12人目のナポレオン』方式はゼロサム
	total := 0
	for _, s := range g.Scores() {
		total += s
	}
	if total != 0 {
		t.Errorf("得点の合計が 0 ではなく %d (ゼロサムのはず)", total)
	}
}

func TestPlayThroughWithLocalRules(t *testing.T) {
	presets := map[string]func(*nrules.Options){
		"ジョーカー最強+請求": func(o *nrules.Options) {
			o.JokerRule = "top"
			o.JokerFree = "on"
			o.JokerLead = "trump"
			o.JokerDemand = "on"
			o.Yoromeki = "return"
			o.MightyExempt = "on"
		},
		"セイムツーなし4人": func(o *nrules.Options) {
			o.Players = 4
			o.SameTwo = "off"
			o.MinBid = 12
			o.Scoring = "wikipedia"
		},
		"6人・逆転スラム": func(o *nrules.Options) {
			o.Players = 6
			o.Slam = "reverse"
			o.JokerRule = "bottom"
			o.JokerFree = "on"
		},
		"最低14枚": func(o *nrules.Options) {
			o.MinBid = 14
			o.SameTwo = "not_trump"
			o.Slam = "double"
		},
	}

	for name, tweak := range presets {
		t.Run(name, func(t *testing.T) {
			engine, err := nrules.New("")
			if err != nil {
				t.Fatal(err)
			}
			o := nrules.DefaultOptions()
			tweak(&o)
			rng := rand.New(rand.NewSource(23))
			g, err := ngame.New(engine, rng, 2, o)
			if err != nil {
				t.Fatal(err)
			}
			autoPlay(t, g, engine, rng)
			if g.Phase() != ngame.PhaseGameOver {
				t.Fatalf("最後まで進みませんでした: %s", g.Phase())
			}
		})
	}
}

// 10 トリックで 20 枚の絵札が全て誰かに渡る。
func TestAllPointCardsAreTaken(t *testing.T) {
	engine, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(5))
	g, err := ngame.New(engine, rng, 1, nrules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	autoPlay(t, g, engine, rng)

	taken, err := g.TakenPoints()
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, n := range taken {
		sum += n
	}
	// 埋め札に絵札が伏せられている場合があるので、20 枚を超えることはない
	if sum > g.MaxPoints() {
		t.Errorf("取られた絵札が %d 枚で 20 枚を超えています", sum)
	}
	if sum < g.MaxPoints()-g.WidowSize() {
		t.Errorf("取られた絵札が %d 枚しかありません (埋め札は %d 枚)", sum, g.WidowSize())
	}
}

// ナポレオンと副官が決まり、精算で副官が明かされる。
func TestNapoleonAndAideAreDecided(t *testing.T) {
	engine, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(17))
	g, err := ngame.New(engine, rng, 1, nrules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	autoPlay(t, g, engine, rng)

	if g.NapoleonSeat() < 0 {
		t.Fatal("ナポレオンが決まっていません")
	}
	if g.Bid() == nil {
		t.Fatal("宣言が残っていません")
	}
	if !g.AideRevealed() {
		t.Error("精算後は副官が明かされているはず")
	}
	if !g.Solo() && g.AideSeat() == g.NapoleonSeat() {
		t.Error("副官がナポレオン自身になっています")
	}
	if g.Trump() != g.Bid().Suit {
		t.Errorf("切り札 %s が宣言 %s と一致していません", g.Trump(), g.Bid().Label())
	}
}

// 手札は ♠♥♦♣ の順に並び、各マークの中では A が右端 (いちばん後ろ) に来る。
func TestHandSortPutsAceOnTheRight(t *testing.T) {
	engine, err := nrules.New("")
	if err != nil {
		t.Fatal(err)
	}
	g, err := ngame.New(engine, rand.New(rand.NewSource(7)), 1, nrules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	suitOrder := map[string]int{"s": 0, "h": 1, "d": 2, "c": 3, cards.JokerSuit: 4}
	rankOrder := map[string]int{
		"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9,
		"10": 10, "j": 11, "q": 12, "k": 13, "a": 14,
	}

	hand := g.Hand()
	if len(hand) < 2 {
		t.Fatalf("手札が %d 枚しかありません", len(hand))
	}
	for i := 1; i < len(hand); i++ {
		prev, cur := hand[i-1], hand[i]
		if suitOrder[prev.Suit] > suitOrder[cur.Suit] {
			t.Fatalf("マークの並びが ♠♥♦♣ の順ではありません: %s の後に %s",
				cards.Label(prev), cards.Label(cur))
		}
		if prev.Suit == cur.Suit && rankOrder[prev.Rank] > rankOrder[cur.Rank] {
			t.Errorf("同じマークの中は弱い順のはずが、%s の後に %s",
				cards.Label(prev), cards.Label(cur))
		}
	}
}
