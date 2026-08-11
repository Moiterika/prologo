package main

// rules_test.go — Prolog 側のルール (hearts.pl とその上のレイヤ) の検証。

import (
	"math/rand"
	"testing"
)

func c(suit, rank string) Card { return Card{Suit: suit, Rank: rank} }

func newTestRules(t *testing.T) *Rules {
	t.Helper()
	r, err := NewRules("")
	if err != nil {
		t.Fatalf("Prolog の読み込みに失敗: %v", err)
	}
	return r
}

func cardSet(cards []Card) map[Card]bool {
	m := make(map[Card]bool, len(cards))
	for _, c := range cards {
		m[c] = true
	}
	return m
}

func TestConfigFromHeartsPl(t *testing.T) {
	r := newTestRules(t)

	if n, err := r.TotalPlayers(); err != nil || n != 4 {
		t.Fatalf("total_players = %d, %v; want 4", n, err)
	}
	if s, err := r.HandSize(); err != nil || s != 13 {
		t.Fatalf("hand_size = %d, %v; want 13", s, err)
	}
	deck, err := r.Deck()
	if err != nil || len(deck) != 52 {
		t.Fatalf("deck の枚数 = %d, %v; want 52", len(deck), err)
	}
	if total, err := r.TotalRoundPoints(); err != nil || total != 26 {
		t.Fatalf("total_round_points = %d, %v; want 26", total, err)
	}
	if open, err := r.OpeningCard(); err != nil || open != c("c", "2") {
		t.Fatalf("opening_card = %v, %v; want ♣2", open, err)
	}
}

func TestCardPoints(t *testing.T) {
	r := newTestRules(t)
	cases := []struct {
		card Card
		want int
	}{
		{c("s", "q"), 13},
		{c("h", "2"), 1},
		{c("h", "a"), 1},
		{c("s", "k"), 0},
		{c("c", "2"), 0},
	}
	for _, tc := range cases {
		got, err := r.CardPoint(tc.card)
		if err != nil || got != tc.want {
			t.Errorf("card_point(%v) = %d, %v; want %d", tc.card, got, err, tc.want)
		}
	}
}

// マストフォロー: 台札のスートを持っていたらそれしか出せない。
func TestMustFollowSuit(t *testing.T) {
	r := newTestRules(t)
	hand := []Card{c("c", "5"), c("c", "9"), c("h", "3"), c("s", "q")}
	lead := c("c", "2")

	legal, err := r.LegalCards(hand, &lead, 5, true)
	if err != nil {
		t.Fatal(err)
	}
	got := cardSet(legal)
	if len(got) != 2 || !got[c("c", "5")] || !got[c("c", "9")] {
		t.Fatalf("legal_cards = %v; クラブ 2 枚だけのはず", legal)
	}
}

// 台札のスートが無ければ何でも出せる (スートを切れる)。
func TestVoidSuitCanPlayAnything(t *testing.T) {
	r := newTestRules(t)
	hand := []Card{c("h", "3"), c("s", "q"), c("d", "7")}
	lead := c("c", "2")

	legal, err := r.LegalCards(hand, &lead, 5, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 3 {
		t.Fatalf("legal_cards = %v; 手札全部が出せるはず", legal)
	}
}

// 第1トリックのリードは必ず ♣2。
func TestFirstTrickMustLeadTwoOfClubs(t *testing.T) {
	r := newTestRules(t)
	hand := []Card{c("c", "2"), c("c", "9"), c("h", "3"), c("s", "a")}

	legal, err := r.LegalCards(hand, nil, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != c("c", "2") {
		t.Fatalf("legal_cards = %v; ♣2 だけのはず", legal)
	}
}

// 第1トリックでは点数札 (♥ と ♠Q) を出せない。
func TestFirstTrickNoPointCards(t *testing.T) {
	r := newTestRules(t)
	lead := c("c", "2")

	// クラブが無く、ハート・♠Q・ダイヤを持っている状況
	hand := []Card{c("h", "3"), c("s", "q"), c("d", "7")}
	legal, err := r.LegalCards(hand, &lead, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 1 || legal[0] != c("d", "7") {
		t.Fatalf("legal_cards = %v; ♦7 だけのはず", legal)
	}

	// 点数札しか持っていなければ出さざるを得ない
	onlyPoints := []Card{c("h", "3"), c("s", "q")}
	legal, err = r.LegalCards(onlyPoints, &lead, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 2 {
		t.Fatalf("legal_cards = %v; 点数札しか無いので 2 枚とも出せるはず", legal)
	}
}

// ハートブレイク前はハートからリードできない。
func TestCannotLeadHeartsBeforeBreak(t *testing.T) {
	r := newTestRules(t)
	hand := []Card{c("h", "3"), c("h", "k"), c("d", "7"), c("s", "q")}

	legal, err := r.LegalCards(hand, nil, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	got := cardSet(legal)
	if len(got) != 2 || !got[c("d", "7")] || !got[c("s", "q")] {
		t.Fatalf("legal_cards = %v; ♦7 と ♠Q だけのはず (♠Q はリード可)", legal)
	}

	// ブレイク後は全部リードできる
	legal, err = r.LegalCards(hand, nil, 4, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 4 {
		t.Fatalf("legal_cards = %v; ブレイク後は全部出せるはず", legal)
	}
}

// 手札がハートだけならブレイク前でもリードできる。
func TestHeartsOnlyHandMayLead(t *testing.T) {
	r := newTestRules(t)
	hand := []Card{c("h", "3"), c("h", "k")}

	legal, err := r.LegalCards(hand, nil, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(legal) != 2 {
		t.Fatalf("legal_cards = %v; ハートしか無いので 2 枚とも出せるはず", legal)
	}
}

// トリックの勝者は台札スートの最強カード。
func TestDetermineWinner(t *testing.T) {
	r := newTestRules(t)

	trick := []Card{c("c", "5"), c("c", "k"), c("h", "a"), c("s", "q")}
	win, err := r.DetermineWinner(trick, "c")
	if err != nil {
		t.Fatal(err)
	}
	if win != c("c", "k") {
		t.Fatalf("determine_winner = %v; want ♣K (♥A や ♠Q は台札スートでないので無関係)", win)
	}
}

// スラム (1人で全失点を独占)。
func TestSlam(t *testing.T) {
	r := newTestRules(t)

	var allPoints []Card
	for _, rank := range []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "j", "q", "k", "a"} {
		allPoints = append(allPoints, c("h", rank))
	}
	allPoints = append(allPoints, c("s", "q"))

	taken := [][]Card{{c("c", "3")}, allPoints, {c("d", "5")}, nil}
	final, slammer, err := r.RoundScores(taken)
	if err != nil {
		t.Fatal(err)
	}
	if slammer != 1 {
		t.Fatalf("slammer = %d; want 1", slammer)
	}
	want := []int{26, 0, 26, 26}
	for i := range want {
		if final[i] != want[i] {
			t.Fatalf("final = %v; want %v", final, want)
		}
	}
}

// スラムでない通常精算。
func TestNormalScoring(t *testing.T) {
	r := newTestRules(t)

	taken := [][]Card{
		{c("h", "2"), c("h", "3")},
		{c("s", "q")},
		nil,
		{c("c", "4")},
	}
	final, slammer, err := r.RoundScores(taken)
	if err != nil {
		t.Fatal(err)
	}
	if slammer != -1 {
		t.Fatalf("slammer = %d; want -1", slammer)
	}
	want := []int{2, 13, 0, 0}
	for i := range want {
		if final[i] != want[i] {
			t.Fatalf("final = %v; want %v", final, want)
		}
	}
}

// hearts.pl の validate_deal/1。
func TestValidateDeal(t *testing.T) {
	r := newTestRules(t)
	deck, err := r.Deck()
	if err != nil {
		t.Fatal(err)
	}

	hands := [][]Card{deck[0:13], deck[13:26], deck[26:39], deck[39:52]}
	if ok, err := r.ValidateDeal(hands); err != nil || !ok {
		t.Fatalf("正しい配札が validate_deal を通りません (%v, %v)", ok, err)
	}

	// 1 枚を重複させると通らないはず
	bad := [][]Card{append([]Card(nil), deck[0:13]...), deck[13:26], deck[26:39], deck[39:52]}
	bad[0][0] = deck[13]
	if ok, _ := r.ValidateDeal(bad); ok {
		t.Fatal("重複のある配札が validate_deal を通ってしまいました")
	}
}

// パス方向は 左 → 右 → 向かい → 交換なし の 4 周期。
func TestPassDirection(t *testing.T) {
	r := newTestRules(t)

	want := map[int]string{1: "left", 2: "right", 3: "across", 4: "hold", 5: "left"}
	for round, dir := range want {
		got, err := r.PassDirection(round)
		if err != nil || got != dir {
			t.Errorf("pass_direction(%d) = %q, %v; want %q", round, got, err, dir)
		}
	}

	targets := map[string]int{"left": 1, "right": 3, "across": 2, "hold": 0}
	for dir, to := range targets {
		got, err := r.PassTarget(0, dir, 4)
		if err != nil || got != to {
			t.Errorf("pass_target(0, %s, 4) = %d, %v; want %d", dir, got, err, to)
		}
	}
}

// CPU の AI が常に合法手を返し、1 ラウンドが破綻なく終わることを確認する。
// 失点の合計は必ず 26 点になる。
func TestAIPlaysFullRoundLegally(t *testing.T) {
	r := newTestRules(t)
	rng := rand.New(rand.NewSource(2024))

	for round := 0; round < 20; round++ {
		deck, err := r.Deck()
		if err != nil {
			t.Fatal(err)
		}
		rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

		hands := [][]Card{deck[0:13], deck[13:26], deck[26:39], deck[39:52]}
		for i := range hands {
			hands[i] = append([]Card(nil), hands[i]...)
		}

		opening, err := r.OpeningCard()
		if err != nil {
			t.Fatal(err)
		}
		leader := -1
		for i, hand := range hands {
			for _, card := range hand {
				if card == opening {
					leader = i
				}
			}
		}
		if leader < 0 {
			t.Fatal("開始カードの持ち主が見つかりません")
		}

		taken := make([][]Card, 4)
		broken := false

		for trickNo := 1; trickNo <= 13; trickNo++ {
			var (
				played  []Card
				players []int
				lead    *Card
			)
			for k := 0; k < 4; k++ {
				pi := (leader + k) % 4

				card, err := r.ChooseCard(hands[pi], lead, played, trickNo, broken)
				if err != nil {
					t.Fatalf("round %d trick %d: choose_card が失敗: %v", round, trickNo, err)
				}
				ok, err := r.IsLegal(hands[pi], lead, trickNo, broken, card)
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					t.Fatalf("round %d trick %d: AI が非合法手 %v を選びました (手札 %v, 台札 %v)",
						round, trickNo, card, hands[pi], lead)
				}

				hands[pi] = removeCard(hands[pi], card)
				played = append(played, card)
				players = append(players, pi)
				if k == 0 {
					cc := card
					lead = &cc
				}
			}

			win, err := r.DetermineWinner(played, lead.Suit)
			if err != nil {
				t.Fatal(err)
			}
			winner := players[indexOfCard(played, win)]
			taken[winner] = append(taken[winner], played...)

			if !broken {
				broken, err = r.ContainsHeart(played)
				if err != nil {
					t.Fatal(err)
				}
			}
			leader = winner
		}

		for i := range hands {
			if len(hands[i]) != 0 {
				t.Fatalf("round %d: プレイヤー %d の手札が残っています: %v", round, i, hands[i])
			}
		}

		final, slammer, err := r.RoundScores(taken)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0
		for _, s := range final {
			sum += s
		}
		wantSum := 26
		if slammer >= 0 {
			wantSum = 26 * 3 // スラム成立時は他 3 人が 26 点ずつ
		}
		if sum != wantSum {
			t.Fatalf("round %d: 失点合計 = %d (slammer=%d); want %d", round, sum, slammer, wantSum)
		}
	}
}
