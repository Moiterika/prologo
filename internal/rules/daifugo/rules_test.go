package daifugo_test

import (
	"strings"
	"testing"

	"hearts/internal/cards"
	"hearts/internal/rules/daifugo"
)

// baseOptions はローカルルールを全て外した素の状態。
//
// 既定値 (DefaultOptions) はいつでも変わりうるので、ルール単体の検証は
// ここから始めて、そのテストが見たいルールだけを足す。
// 既定値そのものの検証は TestDefaultOptions が、既定値で遊べることは
// internal/game/daifugo のテストが受け持つ。
func baseOptions() daifugo.Options {
	o := daifugo.DefaultOptions()
	o.EightCut = "off"
	o.JackBack = "off"
	o.JackBackMode = "forced"
	o.SeqRevolution = "off"
	o.FiveCard = "off"
	o.Bind = "off"
	o.BindStrict = "off"
	o.BindPartial = "off"
	o.TenDiscard = "off"
	o.SevenGive = "off"
	o.SpadeThree = "off"
	o.PassLock = "off"
	o.FinishFlush = "off"
	o.FoulFinish = "off"
	o.MiyakoOchi = "off"
	return o
}

// newEngine は素のルールに tweak を重ねて Prolog を起動する。
func newEngine(t *testing.T, tweak func(*daifugo.Options)) *daifugo.Engine {
	t.Helper()
	e, err := daifugo.New("")
	if err != nil {
		t.Fatalf("Prolog の読み込みに失敗しました: %v", err)
	}
	o := baseOptions()
	if tweak != nil {
		tweak(&o)
	}
	if err := o.Apply(e); err != nil {
		t.Fatalf("ローカルルールの設定に失敗しました: %v", err)
	}
	return e
}

func c(suit, rank string) daifugo.Card { return cards.Card{Suit: suit, Rank: rank} }

func TestDeck(t *testing.T) {
	for _, tc := range []struct {
		jokers int
		want   int
	}{{0, 52}, {1, 53}, {2, 54}} {
		e := newEngine(t, func(o *daifugo.Options) { o.Jokers = tc.jokers })
		deck, err := e.Deck()
		if err != nil {
			t.Fatalf("Deck: %v", err)
		}
		if len(deck) != tc.want {
			t.Errorf("ジョーカー %d 枚のデッキは %d 枚のはずが %d 枚", tc.jokers, tc.want, len(deck))
		}
	}
}

func TestDealSizes(t *testing.T) {
	e := newEngine(t, nil)
	// 5 人に 54 枚 → 11, 11, 11, 11, 10
	sizes, err := e.DealSizes(5, 54)
	if err != nil {
		t.Fatalf("DealSizes: %v", err)
	}
	want := []int{11, 11, 11, 11, 10}
	if len(sizes) != len(want) {
		t.Fatalf("枚数の数が違います: %v", sizes)
	}
	total := 0
	for i, s := range sizes {
		if s != want[i] {
			t.Errorf("%d 番目の枚数が %d ではなく %d", i, want[i], s)
		}
		total += s
	}
	if total != 54 {
		t.Errorf("合計が 54 ではなく %d", total)
	}
}

// 基本の強弱。場に出ているのと同じ形で、より強いものだけ出せる。
func TestResolvePlayBasics(t *testing.T) {
	e := newEngine(t, nil)
	hand := []daifugo.Card{c("s", "5"), c("h", "5"), c("d", "9"), c("c", "3")}

	// 場が空 → 何でも出せる
	field, _, ok, err := e.ResolvePlay(hand, []daifugo.Card{c("d", "9")}, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatalf("場が空なら単騎は出せるはず: ok=%v err=%v", ok, err)
	}
	if field.Kind != "group" || field.Len != 1 {
		t.Errorf("場の形が group/1 ではありません: %+v", field)
	}

	// ♦9 の場に ♣3 は弱いので出せない
	_, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("c", "3")}, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("♦9 に ♣3 は出せてはいけません")
	}

	// 枚数が違えば出せない
	_, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("s", "5"), c("h", "5")}, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("単騎の場にペアは出せてはいけません")
	}
}

// 革命中は強弱が入れ替わる。ジョーカーだけは最強のまま。
func TestRevolutionReversesStrength(t *testing.T) {
	e := newEngine(t, nil)
	hand := []daifugo.Card{c("s", "3"), c("h", "2")}

	// 通常時: ♠3 の場に ♥2 を出せる
	field, _, ok, err := e.ResolvePlay(hand, []daifugo.Card{c("s", "3")}, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	_, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("h", "2")}, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("通常時は ♠3 に ♥2 で勝てるはず")
	}

	// 革命中: ♥2 の場に ♠3 を出せる
	field, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("h", "2")}, daifugo.EmptyField(), true)
	if err != nil || !ok {
		t.Fatal(err)
	}
	_, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("s", "3")}, field, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("革命中は ♥2 に ♠3 で勝てるはず")
	}

	// 革命中でもジョーカーには勝てない
	jokerField, _, ok, err := e.ResolvePlay([]daifugo.Card{daifugo.Joker1},
		[]daifugo.Card{daifugo.Joker1}, daifugo.EmptyField(), true)
	if err != nil || !ok {
		t.Fatal(err)
	}
	_, _, ok, err = e.ResolvePlay(hand, []daifugo.Card{c("s", "3")}, jokerField, true)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("革命中でもジョーカーの単騎には勝てないはず (スペ3返しなし)")
	}
}

// フォーカードで革命。階段革命は設定次第。
func TestRevolutionEffects(t *testing.T) {
	e := newEngine(t, nil)
	quad := []daifugo.Card{c("s", "7"), c("h", "7"), c("d", "7"), c("c", "7")}
	_, fx, ok, err := e.ResolvePlay(quad, quad, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatalf("フォーカードは出せるはず: %v", err)
	}
	if !fx.Revolution {
		t.Error("フォーカードは革命になるはず")
	}

	stair := []daifugo.Card{c("s", "4"), c("s", "5"), c("s", "6"), c("s", "7")}

	off := newEngine(t, func(o *daifugo.Options) { o.SeqRevolution = "off" })
	_, fx, ok, err = off.ResolvePlay(stair, stair, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatalf("4 枚階段は出せるはず: %v", err)
	}
	if fx.Revolution {
		t.Error("階段革命なしの設定では革命にならないはず")
	}

	on := newEngine(t, func(o *daifugo.Options) { o.SeqRevolution = "on" })
	_, fx, ok, err = on.ResolvePlay(stair, stair, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatalf("4 枚階段は出せるはず: %v", err)
	}
	if !fx.Revolution {
		t.Error("階段革命ありの設定では革命になるはず")
	}
}

// 8流し。設定で階段の 8 を対象外にできる。
func TestEightCut(t *testing.T) {
	all := newEngine(t, func(o *daifugo.Options) { o.EightCut = "all" })
	stair := []daifugo.Card{c("s", "7"), c("s", "8"), c("s", "9")}
	_, fx, ok, err := all.ResolvePlay(stair, stair, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if !fx.Cut {
		t.Error("「階段の 8 も含む」設定では流れるはず")
	}

	grp := newEngine(t, func(o *daifugo.Options) { o.EightCut = "group_only" })
	_, fx, ok, err = grp.ResolvePlay(stair, stair, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if fx.Cut {
		t.Error("「階段の 8 は対象外」設定では流れないはず")
	}
	single := []daifugo.Card{c("s", "8")}
	_, fx, ok, err = grp.ResolvePlay(single, single, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if !fx.Cut {
		t.Error("単騎の 8 は「対象外」設定でも流れるはず")
	}

	off := newEngine(t, func(o *daifugo.Options) { o.EightCut = "off" })
	_, fx, ok, err = off.ResolvePlay(single, single, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if fx.Cut {
		t.Error("8流しなしの設定では流れないはず")
	}
}

// 階段はマークが揃っていて、数字が連続していること。
func TestSequence(t *testing.T) {
	e := newEngine(t, nil)
	good := []daifugo.Card{c("s", "9"), c("s", "10"), c("s", "j")}
	_, field, ok, err := func() (daifugo.Field, daifugo.Field, bool, error) {
		f, _, ok, err := e.ResolvePlay(good, good, daifugo.EmptyField(), false)
		return f, f, ok, err
	}()
	if err != nil || !ok {
		t.Fatalf("♠9♠10♠J は階段として出せるはず: %v", err)
	}
	if field.Kind != "seq" || field.Len != 3 {
		t.Errorf("階段として認識されていません: %+v", field)
	}

	// マークが揃っていない
	bad := []daifugo.Card{c("s", "9"), c("h", "10"), c("s", "j")}
	_, _, ok, err = e.ResolvePlay(bad, bad, daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("マークが揃っていない 3 枚は出せてはいけません")
	}

	// 数字が飛んでいる
	gap := []daifugo.Card{c("s", "9"), c("s", "j"), c("s", "k")}
	_, _, ok, err = e.ResolvePlay(gap, gap, daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("数字が飛んでいる 3 枚は出せてはいけません")
	}
}

// ジョーカーは階段の穴を埋められる。
func TestJokerFillsSequence(t *testing.T) {
	e := newEngine(t, nil)
	hand := []daifugo.Card{c("s", "9"), daifugo.Joker1, c("s", "j")}
	f, _, ok, err := e.ResolvePlay(hand, hand, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatalf("♠9 JOKER ♠J は階段として出せるはず: %v", err)
	}
	if f.Kind != "seq" || f.Len != 3 || f.Jokers != 1 {
		t.Errorf("階段として認識されていません: %+v", f)
	}
}

// スペ3返し。単騎のジョーカーに ♠3 だけが勝ち、場が流れる。
func TestSpadeThreeReturn(t *testing.T) {
	joker := []daifugo.Card{daifugo.Joker1}
	spade3 := []daifugo.Card{c("s", "3")}

	on := newEngine(t, func(o *daifugo.Options) { o.SpadeThree = "on" })
	field, _, ok, err := on.ResolvePlay(joker, joker, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	_, fx, ok, err := on.ResolvePlay(spade3, spade3, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("スペ3返しありの設定では ♠3 で勝てるはず")
	}
	if !fx.Cut {
		t.Error("スペ3返しでは場が流れるはず")
	}

	// ♦3 では返せない
	d3 := []daifugo.Card{c("d", "3")}
	_, _, ok, err = on.ResolvePlay(d3, d3, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("♦3 ではジョーカーに勝てません")
	}

	off := newEngine(t, func(o *daifugo.Options) { o.SpadeThree = "off" })
	field, _, _, err = off.ResolvePlay(joker, joker, daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	_, _, ok, err = off.ResolvePlay(spade3, spade3, field, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("スペ3返しなしの設定では ♠3 で勝ててはいけません")
	}
}

// 縛り。1 つ前と同じマーク構成が出たら、以降そのマークでしか出せない。
func TestBind(t *testing.T) {
	e := newEngine(t, func(o *daifugo.Options) { o.Bind = "on" })

	f1, _, ok, err := e.ResolvePlay([]daifugo.Card{c("h", "5")}, []daifugo.Card{c("h", "5")},
		daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f1.Bound) != 0 {
		t.Errorf("1 手目では縛りは発生しません: %+v", f1)
	}

	// ♥5 に ♥9 → ♥ で縛り
	f2, _, ok, err := e.ResolvePlay([]daifugo.Card{c("h", "9")}, []daifugo.Card{c("h", "9")}, f1, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f2.Bound) != 1 || f2.Bound[0] != "h" {
		t.Fatalf("♥ で縛られるはず: %+v", f2)
	}

	// 以降 ♥ 以外は出せない
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("s", "k")}, []daifugo.Card{c("s", "k")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("♥ で縛られているので ♠K は出せません")
	}
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("h", "k")}, []daifugo.Card{c("h", "k")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("♥K なら出せるはず")
	}
	// ジョーカーは縛られたマークの代わりになる
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{daifugo.Joker1}, []daifugo.Card{daifugo.Joker1}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("ジョーカーは縛りの代わりになるはず")
	}
}

// 激縛り。縛り発生時に数字が 1 つ上がっていたら、次は必ずその次の数字。
func TestStrongBind(t *testing.T) {
	e := newEngine(t, func(o *daifugo.Options) { o.Bind = "on"; o.BindStrict = "on" })

	f1, _, _, err := e.ResolvePlay([]daifugo.Card{c("h", "5")}, []daifugo.Card{c("h", "5")},
		daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	f2, _, ok, err := e.ResolvePlay([]daifugo.Card{c("h", "6")}, []daifugo.Card{c("h", "6")}, f1, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if !f2.Strict {
		t.Fatalf("♥5 → ♥6 で激縛りになるはず: %+v", f2)
	}
	// ♥7 だけ出せる
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("h", "7")}, []daifugo.Card{c("h", "7")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("♥7 は出せるはず")
	}
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("h", "8")}, []daifugo.Card{c("h", "8")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("激縛り中に ♥8 は出せません")
	}
}

// 反則上がり。
func TestFoulFinish(t *testing.T) {
	on := newEngine(t, func(o *daifugo.Options) {
		o.FoulFinish = "on"
		o.EightCut = "all"
	})
	for _, tc := range []struct {
		name string
		play []daifugo.Card
		rev  bool
		want bool
	}{
		{"通常時の 2", []daifugo.Card{c("s", "2")}, false, true},
		{"革命時の 2", []daifugo.Card{c("s", "2")}, true, false},
		{"革命時の 3", []daifugo.Card{c("s", "3")}, true, true},
		{"通常時の 3", []daifugo.Card{c("s", "3")}, false, false},
		{"ジョーカー", []daifugo.Card{daifugo.Joker1}, false, true},
		{"8流し採用時の 8", []daifugo.Card{c("s", "8")}, false, true},
		{"普通のカード", []daifugo.Card{c("s", "5")}, false, false},
	} {
		got, err := on.FoulFinish(tc.play, tc.rev)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s: 反則上がり判定が %v ではなく %v", tc.name, tc.want, got)
		}
	}

	off := newEngine(t, func(o *daifugo.Options) { o.FoulFinish = "off" })
	got, err := off.FoulFinish([]daifugo.Card{c("s", "2")}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("反則上がりなしの設定では反則にならないはず")
	}
}

// CPU は必ず合法手かパスを返す。
func TestChoosePlayIsAlwaysLegal(t *testing.T) {
	e := newEngine(t, nil)
	// スートが散らばった、実戦に近い手札にする。
	// 同じスートが 13 枚揃った手札は合法手の数が跳ね上がり、
	// 配られ得ないわりにテストだけが遅くなる。
	hand := []daifugo.Card{
		c("s", "3"), c("h", "5"), c("d", "9"), c("c", "j"), c("s", "q"),
		c("h", "2"), c("d", "4"), c("c", "7"), c("s", "10"), c("h", "k"),
		c("d", "a"), c("c", "6"), daifugo.Joker1,
	}

	field := daifugo.EmptyField()
	for seed := 0; seed < 30; seed++ {
		play, pass, err := e.ChoosePlay(hand, field, false, true, seed)
		if err != nil {
			t.Fatalf("seed=%d: %v", seed, err)
		}
		if pass {
			if field.Empty {
				t.Errorf("seed=%d: 親番でパスしてはいけません", seed)
			}
			continue
		}
		if len(play) == 0 {
			t.Fatalf("seed=%d: パスでないのにカードが空です", seed)
		}
		newField, _, ok, err := e.ResolvePlay(hand, play, field, false)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("seed=%d: CPU が非合法な手を選びました: %v", seed, play)
		}
		field = newField
	}
}

// 称号・交換・得点。
func TestTitlesAndExchanges(t *testing.T) {
	e := newEngine(t, nil)

	titles, err := e.Titles(4)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"daifugo", "fugo", "hinmin", "daihinmin"}
	if len(titles) != len(want) {
		t.Fatalf("4 人の称号は %v のはずが %v", want, titles)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("%d 位の称号が %s ではなく %s", i+1, want[i], titles[i])
		}
	}

	titles, err = e.Titles(3)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"daifugo", "heimin", "daihinmin"}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("3 人戦 %d 位の称号が %s ではなく %s", i+1, want[i], titles[i])
		}
	}

	ex, err := e.Exchanges(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex) != 2 {
		t.Fatalf("4 人戦の交換は 2 組のはずが %v", ex)
	}
	if ex[0] != (daifugo.Exchange{High: 1, Low: 4, Count: 2}) {
		t.Errorf("大富豪と大貧民の交換が 2 枚ではありません: %+v", ex[0])
	}
	if ex[1] != (daifugo.Exchange{High: 2, Low: 3, Count: 1}) {
		t.Errorf("富豪と貧民の交換が 1 枚ではありません: %+v", ex[1])
	}

	ex, err = e.Exchanges(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex) != 1 {
		t.Errorf("3 人戦の交換は 1 組のはずが %v", ex)
	}
}

// 場の説明文。ランクは 10 までが整数、J 以降がアトムなので両方を確かめる。
func TestFieldLabel(t *testing.T) {
	e := newEngine(t, nil)
	for _, tc := range []struct {
		name string
		play []daifugo.Card
		rev  bool
		want string
	}{
		{"単騎", []daifugo.Card{c("d", "6")}, false, "6 1枚"},
		{"整数ランクのペア", []daifugo.Card{c("s", "10"), c("h", "10")}, false, "10 のペア"},
		{"アトムランクのペア", []daifugo.Card{c("s", "q"), c("h", "q")}, false, "Q のペア"},
		{"フォーカード", []daifugo.Card{c("s", "7"), c("h", "7"), c("d", "7"), c("c", "7")}, false, "7 の 4 枚"},
		{"階段", []daifugo.Card{c("s", "9"), c("s", "10"), c("s", "j")}, false, "J までの 3 枚階段"},
		{"ジョーカー単騎", []daifugo.Card{daifugo.Joker1}, false, "JOKER 1枚"},
		{"革命中の単騎", []daifugo.Card{c("d", "6")}, true, "6 1枚"},
	} {
		field, _, ok, err := e.ResolvePlay(tc.play, tc.play, daifugo.EmptyField(), tc.rev)
		if err != nil || !ok {
			t.Fatalf("%s: 出せるはず: ok=%v err=%v", tc.name, ok, err)
		}
		got, err := e.FieldLabel(field, tc.rev)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: 説明文が %q ではなく %q", tc.name, tc.want, got)
		}
	}

	got, err := e.FieldLabel(daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "場は空" {
		t.Errorf("場が空のときの説明文が %q", got)
	}
}

// 片方縛り。縛りの一種なので、縛り自体を使わない設定では効かない。
func TestPartialBind(t *testing.T) {
	e := newEngine(t, func(o *daifugo.Options) { o.Bind = "on"; o.BindPartial = "on" })

	// 本の例: ♥5♣5 に ♠9♣9 → ♣ が片方縛り
	first := []daifugo.Card{c("h", "5"), c("c", "5")}
	f1, _, ok, err := e.ResolvePlay(first, first, daifugo.EmptyField(), false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	second := []daifugo.Card{c("s", "9"), c("c", "9")}
	f2, _, ok, err := e.ResolvePlay(second, second, f1, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f2.Bound) != 1 || f2.Bound[0] != "c" {
		t.Fatalf("♣ で縛られるはず: %+v", f2)
	}

	// 以降は ♣ を含むペアしか出せない
	bad := []daifugo.Card{c("s", "10"), c("h", "10")}
	_, _, ok, err = e.ResolvePlay(bad, bad, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("♣ を含まないので出せません")
	}
	good := []daifugo.Card{c("c", "10"), c("h", "10")}
	_, _, ok, err = e.ResolvePlay(good, good, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("♣ を含むペアなら出せるはず")
	}

	// 縛り自体を使わなければ、片方縛りだけ入れても効かない
	off := newEngine(t, func(o *daifugo.Options) { o.Bind = "off"; o.BindPartial = "on" })
	f1, _, _, err = off.ResolvePlay(first, first, daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	f2, _, ok, err = off.ResolvePlay(second, second, f1, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f2.Bound) != 0 {
		t.Errorf("縛りなしの設定では縛られないはず: %+v", f2)
	}
}

// 激縛りと片方縛りは互いに独立していて、両方入れられる。
func TestStrictAndPartialTogether(t *testing.T) {
	e := newEngine(t, func(o *daifugo.Options) {
		o.Bind = "on"
		o.BindStrict = "on"
		o.BindPartial = "on"
	})
	first := []daifugo.Card{c("h", "5"), c("c", "5")}
	f1, _, _, err := e.ResolvePlay(first, first, daifugo.EmptyField(), false)
	if err != nil {
		t.Fatal(err)
	}
	// マークが一部一致 (♣) で、数字はちょうど 1 つ上 → 片方縛り + 激縛り
	second := []daifugo.Card{c("s", "6"), c("c", "6")}
	f2, _, ok, err := e.ResolvePlay(second, second, f1, false)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f2.Bound) != 1 || f2.Bound[0] != "c" {
		t.Errorf("♣ で縛られるはず: %+v", f2)
	}
	if !f2.Strict {
		t.Errorf("数字が 1 つ上がったので激縛りになるはず: %+v", f2)
	}
	// ♣ を含む 7 のペアだけ出せる
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("c", "7"), c("h", "7")},
		[]daifugo.Card{c("c", "7"), c("h", "7")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("♣ を含む 7 のペアは出せるはず")
	}
	_, _, ok, err = e.ResolvePlay([]daifugo.Card{c("c", "8"), c("h", "8")},
		[]daifugo.Card{c("c", "8"), c("h", "8")}, f2, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("激縛り中なので 8 は出せません")
	}
}

// 10捨て・7渡しの枚数。階段に含まれる 10 や 7 も数える。
func TestTenDiscardAndSevenGiveCounts(t *testing.T) {
	on := newEngine(t, func(o *daifugo.Options) { o.TenDiscard = "on"; o.SevenGive = "on" })
	for _, tc := range []struct {
		name      string
		play      []daifugo.Card
		wantTen   int
		wantSeven int
	}{
		{"10 単騎", []daifugo.Card{c("s", "10")}, 1, 0},
		{"10 のペア", []daifugo.Card{c("s", "10"), c("h", "10")}, 2, 0},
		{"階段に 10 と 7 を含む", []daifugo.Card{c("s", "7"), c("s", "8"), c("s", "9"), c("s", "10")}, 1, 1},
		{"7 のスリーカード", []daifugo.Card{c("s", "7"), c("h", "7"), c("d", "7")}, 0, 3},
		{"関係ない手", []daifugo.Card{c("s", "5")}, 0, 0},
		{"ジョーカーは数えない", []daifugo.Card{daifugo.Joker1}, 0, 0},
	} {
		got, err := on.TenDiscardCount(tc.play)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.wantTen {
			t.Errorf("%s: 10捨ての枚数が %d ではなく %d", tc.name, tc.wantTen, got)
		}
		got, err = on.SevenGiveCount(tc.play)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.wantSeven {
			t.Errorf("%s: 7渡しの枚数が %d ではなく %d", tc.name, tc.wantSeven, got)
		}
	}

	// 使わない設定では 0
	off := newEngine(t, func(o *daifugo.Options) { o.TenDiscard = "off"; o.SevenGive = "off" })
	n, err := off.TenDiscardCount([]daifugo.Card{c("s", "10"), c("h", "10")})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("10捨てなしの設定では 0 のはずが %d", n)
	}
	n, err = off.SevenGiveCount([]daifugo.Card{c("s", "7")})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("7渡しなしの設定では 0 のはずが %d", n)
	}
}

// 10捨て・7渡しで手札が足りないときの反則上がり。
func TestFoulShortage(t *testing.T) {
	on := newEngine(t, func(o *daifugo.Options) { o.FoulFinish = "on" })
	for _, tc := range []struct {
		required, remaining int
		want                bool
	}{
		{2, 0, true},
		{2, 1, true},
		{2, 2, false},
		{1, 3, false},
		{0, 0, false},
	} {
		got, err := on.FoulShortage(tc.required, tc.remaining)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%d 枚必要で手札 %d 枚のとき、反則判定が %v ではなく %v",
				tc.required, tc.remaining, tc.want, got)
		}
	}

	off := newEngine(t, func(o *daifugo.Options) { o.FoulFinish = "off" })
	got, err := off.FoulShortage(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("反則上がりなしの設定では反則になりません")
	}
}

// Jバックの任意適用。
func TestJackBackOptional(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tweak func(*daifugo.Options)
		want  bool
	}{
		{"Jバックなし", func(o *daifugo.Options) { o.JackBack = "off"; o.JackBackMode = "optional" }, false},
		{"強制適用", func(o *daifugo.Options) { o.JackBack = "all"; o.JackBackMode = "forced" }, false},
		{"任意適用", func(o *daifugo.Options) { o.JackBack = "all"; o.JackBackMode = "optional" }, true},
	} {
		e := newEngine(t, tc.tweak)
		got, err := e.JackBackOptional()
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s: %v ではなく %v", tc.name, tc.want, got)
		}
	}
}

// 既定のローカルルールと、画面の初期表示が一致している。
func TestDefaultOptions(t *testing.T) {
	wantChoice := map[string]string{
		"players":        "4",
		"jokers":         "2",
		"first_play":     "diamond3_free",
		"eight_cut":      "all",
		"jack_back":      "all",
		"seq_revolution": "on",
		"five_card":      "on",
		"bind":           "on",
		"ten_discard":    "on",
		"seven_give":     "off",
		"spade_three":    "on",
		"pass_lock":      "off",
		"finish_flush":   "off",
		"foul_finish":    "on",
		"miyako_ochi":    "on",
	}
	wantToggle := map[string]bool{
		"jack_back_optional": true,
		"bind_strict":        true,
		"bind_partial":       true,
	}

	seenChoice, seenToggle := map[string]bool{}, map[string]bool{}
	for _, g := range daifugo.DefaultOptions().Groups() {
		w, ok := wantChoice[g.Name]
		if !ok {
			t.Errorf("知らない問い %q が増えています", g.Name)
			continue
		}
		seenChoice[g.Name] = true
		if g.Selected != w {
			t.Errorf("%s の既定が %q ではなく %q", g.Name, w, g.Selected)
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

// 選択肢は「なし」を先頭に置く。
func TestChoicesStartWithOff(t *testing.T) {
	for _, g := range daifugo.DefaultOptions().Groups() {
		if len(g.Choices) == 0 {
			t.Errorf("%s に選択肢がありません", g.Name)
			continue
		}
		hasOff := false
		for _, c := range g.Choices {
			if c.Label == "なし" || strings.HasPrefix(c.Label, "なし ") {
				hasOff = true
			}
		}
		if !hasOff {
			continue // 「なし」が無い問い (人数・1 ゲーム目の始め方) は対象外
		}
		if first := g.Choices[0].Label; first != "なし" && !strings.HasPrefix(first, "なし ") {
			t.Errorf("%s の先頭が「なし」ではなく %q", g.Name, first)
		}
	}
}

// 枝分かれは、親の流儀を使わない選択をしているあいだ隠す。
func TestTogglesHiddenWhenParentIsOff(t *testing.T) {
	on := daifugo.DefaultOptions() // 既定では Jバックも縛りも「あり」
	for _, g := range on.Groups() {
		if len(g.Toggles) == 0 {
			continue
		}
		if g.TogglesHideOn == "" {
			t.Errorf("%s に隠す条件がありません", g.Name)
		}
		if g.TogglesHidden() {
			t.Errorf("%s は「あり」なのに枝分かれが隠れています", g.Name)
		}
	}

	off := daifugo.DefaultOptions()
	off.JackBack = "off"
	off.Bind = "off"
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
		"bind":        "on",
		"bind_strict": "on",
		// bind_partial は送らない (チェックを外した状態)
		"jack_back":   "all",
		"ten_discard": "off",
		"seven_give":  "on",
	}
	o := daifugo.ParseOptions(func(k string) string { return form[k] })
	if o.Bind != "on" || o.BindStrict != "on" || o.BindPartial != "off" {
		t.Errorf("縛りの解釈が想定と違います: %+v", o)
	}
	if o.JackBackMode != "forced" {
		t.Errorf("チェックを外した任意適用が forced になっていません: %q", o.JackBackMode)
	}
	if o.TenDiscard != "off" || o.SevenGive != "on" {
		t.Errorf("10捨て・7渡しの解釈が想定と違います: %+v", o)
	}

	// 何も送られてこなければ、ラジオは既定のまま・チェックボックスは全て外れる。
	// 「外した」と「そもそも送られていない」は区別できないので、外した側に倒す。
	want := daifugo.DefaultOptions()
	want.JackBackMode = "forced"
	want.BindStrict = "off"
	want.BindPartial = "off"
	if empty := daifugo.ParseOptions(func(string) string { return "" }); empty != want {
		t.Errorf("空のフォームの解釈が想定と違います:\n got %+v\nwant %+v", empty, want)
	}
}
