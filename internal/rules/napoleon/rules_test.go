package napoleon_test

import (
	"testing"

	"hearts/internal/cards"
	"hearts/internal/rules/napoleon"
)

func newEngine(t *testing.T, tweak func(*napoleon.Options)) *napoleon.Engine {
	t.Helper()
	e, err := napoleon.New("")
	if err != nil {
		t.Fatalf("Prolog の読み込みに失敗しました: %v", err)
	}
	o := napoleon.DefaultOptions()
	if tweak != nil {
		tweak(&o)
	}
	if err := o.Apply(e); err != nil {
		t.Fatalf("ローカルルールの設定に失敗しました: %v", err)
	}
	return e
}

func c(suit, rank string) napoleon.Card { return cards.Card{Suit: suit, Rank: rank} }

func TestDeckAndDeal(t *testing.T) {
	e := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "none" })
	deck, err := e.Deck()
	if err != nil {
		t.Fatal(err)
	}
	if len(deck) != 52 {
		t.Errorf("ジョーカーなしのデッキは 52 枚のはずが %d 枚", len(deck))
	}
	hand, err := e.HandSize()
	if err != nil {
		t.Fatal(err)
	}
	widow, err := e.WidowSize()
	if err != nil {
		t.Fatal(err)
	}
	if hand != 10 || widow != 2 {
		t.Errorf("5 人戦は 10 枚ずつ + 埋め札 2 枚のはずが %d / %d", hand, widow)
	}

	withJoker := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top" })
	deck, err = withJoker.Deck()
	if err != nil {
		t.Fatal(err)
	}
	if len(deck) != 53 {
		t.Errorf("ジョーカーありのデッキは 53 枚のはずが %d 枚", len(deck))
	}
	widow, err = withJoker.WidowSize()
	if err != nil {
		t.Fatal(err)
	}
	if widow != 3 {
		t.Errorf("ジョーカーありの埋め札は 3 枚のはずが %d 枚", widow)
	}
}

func TestPointCards(t *testing.T) {
	e := newEngine(t, nil)
	deck, err := e.Deck()
	if err != nil {
		t.Fatal(err)
	}
	n, err := e.CountPoints(deck)
	if err != nil {
		t.Fatal(err)
	}
	if n != 20 {
		t.Errorf("絵札は 20 枚のはずが %d 枚", n)
	}
}

// 三役と切り札の順序。切り札 ♥ なら 正J=♥J、裏J=♦J。
func TestTrickWinnerBasics(t *testing.T) {
	e := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "off" })

	mainJ, err := e.MainJack("h")
	if err != nil {
		t.Fatal(err)
	}
	if mainJ != c("h", "j") {
		t.Errorf("切り札 ♥ の正ジャックは ♥J のはずが %v", mainJ)
	}
	counterJ, err := e.CounterJack("h")
	if err != nil {
		t.Fatal(err)
	}
	if counterJ != c("d", "j") {
		t.Errorf("切り札 ♥ の裏ジャックは ♦J のはずが %v", counterJ)
	}

	for _, tc := range []struct {
		name  string
		trick []napoleon.Card
		lead  string
		want  napoleon.Card
	}{
		{
			"オールマイティが最強",
			[]napoleon.Card{c("c", "5"), c("h", "a"), c("s", "a"), c("c", "k"), c("c", "3")},
			"c", c("s", "a"),
		},
		{
			"正Jは裏Jより強い",
			[]napoleon.Card{c("c", "5"), c("d", "j"), c("h", "j"), c("c", "k"), c("c", "3")},
			"c", c("h", "j"),
		},
		{
			"裏Jは切り札のAより強い",
			[]napoleon.Card{c("c", "5"), c("d", "j"), c("h", "a"), c("c", "k"), c("c", "3")},
			"c", c("d", "j"),
		},
		{
			"切り札は台札より強い",
			[]napoleon.Card{c("c", "a"), c("h", "3"), c("c", "k"), c("c", "q"), c("c", "9")},
			"c", c("h", "3"),
		},
		{
			"台札以外は必ず負ける",
			[]napoleon.Card{c("c", "5"), c("d", "a"), c("s", "k"), c("c", "6"), c("d", "q")},
			"c", c("c", "6"),
		},
	} {
		got, reason, err := e.TrickWinner(tc.trick, "h", tc.lead)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: 勝ち札が %v ではなく %v (理由 %s)",
				tc.name, cards.Label(tc.want), cards.Label(got), reason)
		}
	}
}

// セイムツー。台札と同じマークだけが出揃い、2 があればその 2 が勝つ。
func TestSameTwo(t *testing.T) {
	on := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "on" })
	// 本の例: ♠6, ♠9, ♠K, ♠8, ♠2 → ♠2 の勝ち (切り札は ♣)
	trick := []napoleon.Card{c("s", "6"), c("s", "9"), c("s", "k"), c("s", "8"), c("s", "2")}
	got, reason, err := on.TrickWinner(trick, "c", "s")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("s", "2") || reason != "same_two" {
		t.Errorf("セイムツーで ♠2 が勝つはずが %v (%s)", cards.Label(got), reason)
	}

	// マークが 1 枚でも違えば発動しない
	mixed := []napoleon.Card{c("s", "6"), c("s", "9"), c("h", "k"), c("s", "8"), c("s", "2")}
	got, reason, err = on.TrickWinner(mixed, "c", "s")
	if err != nil {
		t.Fatal(err)
	}
	if reason == "same_two" {
		t.Errorf("マークが揃っていないのにセイムツーが発動しました: %v", cards.Label(got))
	}

	// 切り札では無効の設定
	notTrump := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "not_trump" })
	got, reason, err = notTrump.TrickWinner(trick, "s", "s")
	if err != nil {
		t.Fatal(err)
	}
	if reason == "same_two" {
		t.Error("「切り札では無効」の設定なのにセイムツーが発動しました")
	}
	if got != c("s", "k") {
		t.Errorf("台札が切り札なら ♠K が勝つはずが %v", cards.Label(got))
	}

	// 無効の設定
	off := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "off" })
	got, _, err = off.TrickWinner(trick, "c", "s")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("s", "k") {
		t.Errorf("セイムツーなしなら ♠K が勝つはずが %v", cards.Label(got))
	}
}

// よろめきとよろめき返し。
func TestYoromeki(t *testing.T) {
	trick := []napoleon.Card{c("d", "5"), c("s", "a"), c("h", "q"), c("d", "9"), c("d", "3")}

	off := newEngine(t, func(o *napoleon.Options) { o.Yoromeki = "off" })
	got, _, err := off.TrickWinner(trick, "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("s", "a") {
		t.Errorf("よろめきなしなら ♠A が勝つはずが %v", cards.Label(got))
	}

	on := newEngine(t, func(o *napoleon.Options) { o.Yoromeki = "on" })
	got, reason, err := on.TrickWinner(trick, "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("h", "q") || reason != "yoromeki" {
		t.Errorf("よろめきで ♥Q が勝つはずが %v (%s)", cards.Label(got), reason)
	}

	withK := []napoleon.Card{c("d", "5"), c("s", "a"), c("h", "q"), c("h", "k"), c("d", "3")}
	ret := newEngine(t, func(o *napoleon.Options) { o.Yoromeki = "return" })
	got, reason, err = ret.TrickWinner(withK, "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("h", "k") || reason != "yoromeki_return" {
		t.Errorf("よろめき返しで ♥K が勝つはずが %v (%s)", cards.Label(got), reason)
	}
	// よろめき返しの設定でも、♥K がいなければ ♥Q が勝つ
	got, _, err = ret.TrickWinner(trick, "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("h", "q") {
		t.Errorf("♥K がなければ ♥Q が勝つはずが %v", cards.Label(got))
	}
}

// ジョーカーの強さはローカルルールで変わる。
func TestJokerPower(t *testing.T) {
	trick := []napoleon.Card{c("c", "5"), napoleon.Joker, c("s", "a"), c("h", "j"), c("c", "3")}
	for _, tc := range []struct {
		mode string
		want napoleon.Card
	}{
		{"top", napoleon.Joker},
		{"second", c("s", "a")},
		{"below", c("s", "a")},
		{"bottom", c("s", "a")},
	} {
		e := newEngine(t, func(o *napoleon.Options) {
			o.JokerRule = tc.mode
			o.SameTwo = "off"
		})
		got, _, err := e.TrickWinner(trick, "h", "c")
		if err != nil {
			t.Fatalf("%s: %v", tc.mode, err)
		}
		if got != tc.want {
			t.Errorf("ジョーカー %s: 勝ち札が %v ではなく %v",
				tc.mode, cards.Label(tc.want), cards.Label(got))
		}
	}

	// 三役未満の設定では、切り札より強く裏Jより弱い
	below := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "below"; o.SameTwo = "off" })
	got, _, err := below.TrickWinner(
		[]napoleon.Card{c("c", "5"), napoleon.Joker, c("h", "a")}, "h", "c")
	if err != nil {
		t.Fatal(err)
	}
	if got != napoleon.Joker {
		t.Errorf("三役未満の設定では切り札 ♥A に勝つはずが %v", cards.Label(got))
	}
	got, _, err = below.TrickWinner(
		[]napoleon.Card{c("c", "5"), napoleon.Joker, c("d", "j")}, "h", "c")
	if err != nil {
		t.Fatal(err)
	}
	if got != c("d", "j") {
		t.Errorf("三役未満の設定では裏J ♦J に負けるはずが %v", cards.Label(got))
	}
}

// マストフォローと三役の拒否権。
func TestLegalCards(t *testing.T) {
	e := newEngine(t, nil)
	hand := []napoleon.Card{c("c", "5"), c("c", "9"), c("h", "3"), c("s", "a")}
	lead := c("c", "k")

	legal, err := e.LegalCards(hand, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 2 {
		t.Fatalf("♣ を 2 枚持っているので ♣ の 2 枚だけのはずが %v", legal)
	}
	for _, card := range legal {
		if card.Suit != "c" {
			t.Errorf("♣ 以外が合法手になっています: %v", cards.Label(card))
		}
	}

	// 台札のマークを持っていなければ何でも出せる
	lead = c("d", "k")
	legal, err = e.LegalCards(hand, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != len(hand) {
		t.Errorf("♦ を持っていないので全部出せるはずが %v", legal)
	}

	// 三役の拒否権: 台札のマークが ♠A (オールマイティ) しかない場合
	exempt := newEngine(t, func(o *napoleon.Options) { o.MightyExempt = "on" })
	hand2 := []napoleon.Card{c("s", "a"), c("c", "5"), c("h", "3")}
	spadeLead := c("s", "k")
	legal, err = exempt.LegalCards(hand2, &spadeLead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 3 {
		t.Errorf("拒否権ありなら温存して他を出せるはずが %v", legal)
	}

	noExempt := newEngine(t, func(o *napoleon.Options) { o.MightyExempt = "off" })
	legal, err = noExempt.LegalCards(hand2, &spadeLead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != c("s", "a") {
		t.Errorf("拒否権なしなら ♠A を出すしかないはずが %v", legal)
	}
}

// ジョーカーの自由出しと切り札請求。
func TestJokerFollowRules(t *testing.T) {
	hand := []napoleon.Card{napoleon.Joker, c("c", "5"), c("h", "3")}
	lead := c("c", "k")

	strict := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top"; o.JokerFree = "off" })
	legal, err := strict.LegalCards(hand, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != c("c", "5") {
		t.Errorf("自由出しなしなら ♣5 だけのはずが %v", legal)
	}

	free := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top"; o.JokerFree = "on" })
	legal, err = free.LegalCards(hand, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 2 {
		t.Errorf("自由出しありなら ♣5 とジョーカーのはずが %v", legal)
	}

	// 切り札請求: ジョーカーが台札なら切り札を出す
	demand := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top"; o.JokerLead = "trump" })
	jokerLead := napoleon.Joker
	legal, err = demand.LegalCards([]napoleon.Card{c("c", "5"), c("h", "3"), c("h", "9")},
		&jokerLead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 2 {
		t.Fatalf("切り札請求なら ♥ の 2 枚だけのはずが %v", legal)
	}
	for _, card := range legal {
		if card.Suit != "h" {
			t.Errorf("切り札以外が合法手になっています: %v", cards.Label(card))
		}
	}

	// 請求なしなら何でも出せる
	noDemand := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top"; o.JokerLead = "none" })
	legal, err = noDemand.LegalCards([]napoleon.Card{c("c", "5"), c("h", "3"), c("h", "9")},
		&jokerLead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 3 {
		t.Errorf("請求なしなら全部出せるはずが %v", legal)
	}
}

// ジョーカー請求 (♠3 が台札)。
func TestJokerDemand(t *testing.T) {
	e := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top"; o.JokerDemand = "on" })
	lead := c("s", "3")
	hand := []napoleon.Card{napoleon.Joker, c("s", "5"), c("h", "3")}
	legal, err := e.LegalCards(hand, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != napoleon.Joker {
		t.Errorf("ジョーカー請求ならジョーカーを出すしかないはずが %v", legal)
	}

	// ジョーカーを持っていなければ普通のフォロー
	legal, err = e.LegalCards([]napoleon.Card{c("s", "5"), c("h", "3")}, &lead, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != c("s", "5") {
		t.Errorf("♠ を持っているので ♠5 だけのはずが %v", legal)
	}
}

// ビッドの強弱。同じ枚数なら ♠ > ♥ > ♦ > ♣。
func TestBidOrder(t *testing.T) {
	e := newEngine(t, nil)
	cur := napoleon.Bid{Count: 10, Suit: "d"}
	for _, tc := range []struct {
		bid  napoleon.Bid
		want bool
	}{
		{napoleon.Bid{Count: 10, Suit: "h"}, true},
		{napoleon.Bid{Count: 10, Suit: "s"}, true},
		{napoleon.Bid{Count: 10, Suit: "c"}, false},
		{napoleon.Bid{Count: 10, Suit: "d"}, false},
		{napoleon.Bid{Count: 11, Suit: "c"}, true},
		{napoleon.Bid{Count: 9, Suit: "s"}, false}, // 最低宣言枚数を下回る
	} {
		got, err := e.BidOK(tc.bid, &cur)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s に対する %s の可否が %v ではなく %v",
				cur.Label(), tc.bid.Label(), tc.want, got)
		}
	}

	// 誰も宣言していなければ最低枚数から
	got, err := e.BidOK(napoleon.Bid{Count: 10, Suit: "c"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("誰も宣言していなければ ♣ 10 枚で宣言できるはず")
	}
	got, err = e.BidOK(napoleon.Bid{Count: 9, Suit: "s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("最低宣言枚数を下回る宣言は通ってはいけません")
	}
}

// 精算。
func TestRoundScores(t *testing.T) {
	twelfth := newEngine(t, nil)
	r, err := twelfth.RoundScores(13, 15, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != "napoleon" || r.Napoleon != 2 || r.Aide != 1 || r.Allied != -1 {
		t.Errorf("宣言達成時の得点が想定と違います: %+v", r)
	}
	r, err = twelfth.RoundScores(13, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != "allied" || r.Napoleon != -2 || r.Aide != -1 || r.Allied != 1 {
		t.Errorf("宣言未達時の得点が想定と違います: %+v", r)
	}
	r, err = twelfth.RoundScores(13, 15, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.Napoleon != 4 || r.Aide != 0 {
		t.Errorf("副官なしのナポレオンは ±4 のはずが %+v", r)
	}

	// Wikipedia 方式: 連合軍の点 = 宣言枚数 − (最低枚数 − 1)
	wiki := newEngine(t, func(o *napoleon.Options) { o.Scoring = "wikipedia" })
	r, err = wiki.RoundScores(13, 15, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Allied != -4 || r.Aide != 4 || r.Napoleon != 8 {
		t.Errorf("Wikipedia 方式の得点が想定と違います: %+v", r)
	}

	// 20 枚独占
	double := newEngine(t, func(o *napoleon.Options) { o.Slam = "double" })
	r, err = double.RoundScores(13, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != "napoleon" || r.Napoleon != 4 {
		t.Errorf("2 倍の設定で 20 枚独占したときの得点が想定と違います: %+v", r)
	}
	reverse := newEngine(t, func(o *napoleon.Options) { o.Slam = "reverse" })
	r, err = reverse.RoundScores(13, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != "allied" || r.Napoleon != -4 {
		t.Errorf("逆転の設定で 20 枚独占したときの得点が想定と違います: %+v", r)
	}
	// 連合軍が 20 枚独占 (ナポレオン軍が 0 枚) した場合も逆転する
	r, err = reverse.RoundScores(13, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome != "napoleon" {
		t.Errorf("連合軍の 20 枚独占も逆転するはずが %+v", r)
	}
}

// CPU は必ず合法手を返す。
func TestChooseCardIsAlwaysLegal(t *testing.T) {
	e := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "top" })
	hand := []napoleon.Card{c("c", "5"), c("c", "9"), c("h", "3"), c("s", "a"), napoleon.Joker}
	lead := c("c", "k")
	for seed := 0; seed < 20; seed++ {
		card, err := e.ChooseCard(hand, []napoleon.Card{lead}, &lead, "h", seed)
		if err != nil {
			t.Fatalf("seed=%d: %v", seed, err)
		}
		ok, err := e.IsLegal(hand, &lead, "h", card)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("seed=%d: CPU が非合法なカード %v を選びました", seed, cards.Label(card))
		}
	}
}

// CPU のビッドは必ず通る宣言かパス。
func TestChooseBidIsAlwaysValid(t *testing.T) {
	e := newEngine(t, nil)
	hand := []napoleon.Card{
		c("s", "a"), c("s", "k"), c("h", "j"), c("h", "9"), c("d", "a"),
		c("d", "3"), c("c", "q"), c("c", "7"), c("h", "2"), c("s", "5"),
	}
	cur := napoleon.Bid{Count: 11, Suit: "d"}
	for seed := 0; seed < 30; seed++ {
		bid, pass, err := e.ChooseBid(hand, &cur, seed)
		if err != nil {
			t.Fatalf("seed=%d: %v", seed, err)
		}
		if pass {
			continue
		}
		ok, err := e.BidOK(bid, &cur)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("seed=%d: CPU が通らない宣言 %s を選びました", seed, bid.Label())
		}
	}
}

// 副官の指名候補は自分の手札に無いカードだけ。
func TestAideCandidates(t *testing.T) {
	e := newEngine(t, nil)
	hand := []napoleon.Card{c("s", "a"), c("h", "j"), c("d", "a"), c("c", "5")}
	cands, err := e.AideCandidates(hand, "h")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) == 0 {
		t.Fatal("副官の候補がありません")
	}
	for _, card := range cands {
		if cards.Contains(hand, card) {
			t.Errorf("自分の手札 %v が副官の候補になっています", cards.Label(card))
		}
	}
}

// =============================================
// 設定画面の選択肢
// =============================================

// 既定のローカルルールが、画面のラジオボタンとチェックボックスにどう映るか。
func TestDefaultOptionGroups(t *testing.T) {
	wantChoice := map[string]string{
		"players":       "5",
		"min_bid":       "10",
		"same_two":      "on",
		"yoromeki":      "on",
		"joker_rule":    "second",
		"mighty_exempt": "on",
		"scoring":       "twelfth",
		"slam":          "normal",
	}
	wantToggle := map[string]bool{
		"same_two_not_trump": true,
		"yoromeki_return":    false,
		"joker_free":         false,
		"joker_lead":         true,
		"joker_demand":       false,
	}

	seenChoice, seenToggle := map[string]bool{}, map[string]bool{}
	for _, g := range napoleon.DefaultOptions().Groups() {
		w, ok := wantChoice[g.Name]
		if !ok {
			t.Errorf("知らない問い %q が増えています", g.Name)
			continue
		}
		seenChoice[g.Name] = true
		if g.Selected != w {
			t.Errorf("%s の既定が %q ではなく %q", g.Name, w, g.Selected)
		}
		if len(g.Toggles) > 0 && g.TogglesHideOn == "" {
			t.Errorf("%s に枝分かれを隠す条件がありません", g.Name)
		}
		if g.TogglesHidden() {
			t.Errorf("%s は既定で「あり」なのに枝分かれが隠れています", g.Name)
		}
		for _, tg := range g.Toggles {
			seenToggle[tg.Name] = true
			if tg.On != wantToggle[tg.Name] {
				t.Errorf("%s の既定のチェックが %v ではなく %v", tg.Name, wantToggle[tg.Name], tg.On)
			}
		}
	}
	for name := range wantChoice {
		if !seenChoice[name] {
			t.Errorf("問い %q が画面にありません", name)
		}
	}
	for name := range wantToggle {
		if !seenToggle[name] {
			t.Errorf("枝分かれ %q が画面にありません", name)
		}
	}
}

// 親を「なし」にしたら、枝分かれのチェックボックスは隠れる。
func TestTogglesHiddenWhenParentIsOff(t *testing.T) {
	off := napoleon.DefaultOptions()
	off.SameTwo = "off"
	off.Yoromeki = "off"
	off.JokerRule = "none"
	for _, g := range off.Groups() {
		if len(g.Toggles) == 0 {
			continue
		}
		if !g.TogglesHidden() {
			t.Errorf("%s は「なし」なので枝分かれを隠すはず", g.Name)
		}
	}
}

// フォームの値から Options を組み立てる。チェックボックスは送られたものだけ on。
func TestParseOptionsToggles(t *testing.T) {
	form := map[string]string{
		"same_two":           "on",
		"same_two_not_trump": "on",
		"yoromeki":           "on",
		// yoromeki_return は送らない (チェックを外した状態)
		"joker_rule":   "top",
		"joker_free":   "on",
		"joker_lead":   "on",
		"min_bid":      "14",
		"scoring":      "wikipedia",
		"joker_demand": "on",
	}
	o := napoleon.ParseOptions(func(k string) string { return form[k] })
	if o.SameTwo != "not_trump" || o.Yoromeki != "on" {
		t.Errorf("セイムツー・よろめきの解釈が想定と違います: %+v", o)
	}
	if o.JokerRule != "top" || o.JokerFree != "on" || o.JokerLead != "trump" || o.JokerDemand != "on" {
		t.Errorf("ジョーカーの解釈が想定と違います: %+v", o)
	}
	if o.MinBid != 14 || o.Scoring != "wikipedia" {
		t.Errorf("最低宣言枚数・得点の解釈が想定と違います: %+v", o)
	}

	// 「なし」を選んだときは、隠れている枝分かれが届いても効かせない。
	hidden := map[string]string{
		"same_two":           "off",
		"same_two_not_trump": "on",
		"joker_rule":         "none",
		"joker_free":         "on",
	}
	o = napoleon.ParseOptions(func(k string) string { return hidden[k] })
	if o.SameTwo != "off" || o.JokerRule != "none" || o.JokerFree != "off" {
		t.Errorf("隠れた枝分かれが効いてしまっています: %+v", o)
	}

	// 何も送られてこなければ、ラジオは既定のまま・チェックボックスは全て外れる。
	// 「外した」と「そもそも送られていない」は区別できないので、外した側に倒す。
	want := napoleon.DefaultOptions()
	want.SameTwo = "off"
	want.Yoromeki = "off"
	want.JokerFree = "off"
	want.JokerLead = "none"
	want.JokerDemand = "off"
	if empty := napoleon.ParseOptions(func(string) string { return "" }); empty != want {
		t.Errorf("空のフォームの解釈が想定と違います:\n got %+v\nwant %+v", empty, want)
	}
}

// CPU のナポレオンは、三役とジョーカーのうち自分が持っていない最強のカードを指名する。
func TestChooseAidePicksStrongest(t *testing.T) {
	// 切り札 ♥ なら 正J=♥J、裏J=♦J。
	plain := []napoleon.Card{c("c", "5"), c("c", "9"), c("d", "7"), c("h", "3")}

	for _, tc := range []struct {
		name  string
		joker string
		hand  []napoleon.Card
		want  napoleon.Card
	}{
		{"何も持っていなければ ♠A", "second", plain, c("s", "a")},
		{"♠A を持っていればジョーカー (2 番手)", "second",
			append(plain, c("s", "a")), napoleon.Joker},
		{"ジョーカー最強ならまずジョーカー", "top", plain, napoleon.Joker},
		{"ジョーカーが三役未満なら ♠A の次は正J", "below",
			append(plain, c("s", "a")), c("h", "j")},
		{"ジョーカーなしの設定では三役だけ", "none",
			append(plain, c("s", "a"), c("h", "j")), c("d", "j")},
		{"三役もジョーカーも全部持っていたら別のカード", "second",
			[]napoleon.Card{c("s", "a"), c("h", "j"), c("d", "j"), napoleon.Joker},
			napoleon.Card{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEngine(t, func(o *napoleon.Options) { o.JokerRule = tc.joker })
			got, err := e.ChooseAide(tc.hand, "h", 0)
			if err != nil {
				t.Fatal(err)
			}
			if cards.Contains(tc.hand, got) {
				t.Fatalf("自分の手札 %s を指名しています", cards.Label(got))
			}
			if tc.want == (napoleon.Card{}) {
				return // 最強札が残っていない場合は「手札以外なら何でもよい」
			}
			if got != tc.want {
				t.Errorf("指名が %s ではなく %s", cards.Label(tc.want), cards.Label(got))
			}
		})
	}

	// Seed が変わっても最強のカードを選ぶ (ここだけはランダムにしない)。
	e := newEngine(t, func(o *napoleon.Options) { o.JokerRule = "second" })
	for seed := 0; seed < 5; seed++ {
		got, err := e.ChooseAide(plain, "h", seed)
		if err != nil {
			t.Fatal(err)
		}
		if got != c("s", "a") {
			t.Errorf("seed=%d: 指名が ♠A ではなく %s", seed, cards.Label(got))
		}
	}
}

// セイムツーが成立しそうな場面では、CPU は台札と同じマークの 2 を出す。
func TestChooseCardPrefersSameTwo(t *testing.T) {
	hand := []napoleon.Card{c("s", "2"), c("s", "q"), c("s", "7"), c("h", "3")}
	lead := c("s", "k")

	// 台札と同じマークだけが出ているあいだは、必ず ♠2
	on := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "on" })
	trick := []napoleon.Card{lead, c("s", "9")}
	for seed := 0; seed < 10; seed++ {
		got, err := on.ChooseCard(hand, trick, &lead, "h", seed)
		if err != nil {
			t.Fatal(err)
		}
		if got != c("s", "2") {
			t.Fatalf("seed=%d: ♠2 を出すはずが %s", seed, cards.Label(got))
		}
	}

	// すでにマークが崩れていたら、セイムツーは成立しないので狙わない
	broken := []napoleon.Card{lead, c("h", "9")}
	if !varies(t, on, hand, broken, &lead, "h") {
		t.Error("マークが崩れているのに ♠2 ばかり出しています")
	}

	// セイムツーなしの設定でも狙わない
	off := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "off" })
	if !varies(t, off, hand, trick, &lead, "h") {
		t.Error("セイムツーなしの設定なのに ♠2 ばかり出しています")
	}

	// 「切り札では無効」の設定で、台札が切り札のマーク (♠) なら狙わない
	notTrump := newEngine(t, func(o *napoleon.Options) { o.SameTwo = "not_trump" })
	if !varies(t, notTrump, hand, trick, &lead, "s") {
		t.Error("台札が切り札のマークなのに ♠2 ばかり出しています")
	}
	// 同じ設定でも、台札が切り札でなければ狙う
	got, err := notTrump.ChooseCard(hand, trick, &lead, "h", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != c("s", "2") {
		t.Errorf("台札が切り札でなければ ♠2 を出すはずが %s", cards.Label(got))
	}
}

// varies は seed を変えると ♠2 以外も選ぶか (＝ 2 を狙っていないか)。
func varies(t *testing.T, e *napoleon.Engine, hand, trick []napoleon.Card, lead *napoleon.Card, trump string) bool {
	t.Helper()
	for seed := 0; seed < 20; seed++ {
		got, err := e.ChooseCard(hand, trick, lead, trump, seed)
		if err != nil {
			t.Fatal(err)
		}
		if got != c("s", "2") {
			return true
		}
	}
	return false
}
