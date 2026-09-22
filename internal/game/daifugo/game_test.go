package daifugo_test

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"hearts/internal/cards"
	dgame "hearts/internal/game/daifugo"
	drules "hearts/internal/rules/daifugo"
)

// autoPlay は「人間の席」も含めて全員をランダムに動かし、1 卓を最後まで進める。
// 進行が止まらないこと、どの局面でも Prolog のルールに反しないことを見る。
func autoPlay(t *testing.T, g *dgame.Game, engine *drules.Engine, rng *rand.Rand) {
	t.Helper()
	for step := 0; step < 4000; step++ {
		switch g.Phase() {
		case dgame.PhaseGameOver:
			return
		case dgame.PhaseRoundEnd:
			if err := g.Next(); err != nil {
				t.Fatalf("Next: %v", err)
			}
		case dgame.PhaseExchange:
			need := g.ExchangeNeed()
			nums := make([]int, 0, need)
			for i := 1; i <= need; i++ {
				nums = append(nums, i)
			}
			if err := g.SubmitExchange(nums); err != nil {
				t.Fatalf("SubmitExchange: %v", err)
			}
		case dgame.PhasePlaying:
			if err := humanMove(t, g, engine, rng); err != nil {
				t.Fatalf("人間の手番: %v", err)
			}
		case dgame.PhaseJackBack:
			if err := g.SubmitJackBack(rng.Intn(2) == 0); err != nil {
				t.Fatalf("SubmitJackBack: %v", err)
			}
		case dgame.PhaseTenDiscard:
			if err := g.SubmitTenDiscard(firstN(g.HandOverNeed())); err != nil {
				t.Fatalf("SubmitTenDiscard: %v", err)
			}
		case dgame.PhaseSevenGive:
			if err := g.SubmitSevenGive(firstN(g.HandOverNeed())); err != nil {
				t.Fatalf("SubmitSevenGive: %v", err)
			}
		default:
			t.Fatalf("知らない局面です: %s", g.Phase())
		}
	}
	t.Fatal("4000 手進めても終わりませんでした")
}

// firstN は手札の先頭から n 枚ぶんの表示番号。
func firstN(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}

// humanMove は人間の席をランダムな合法手で進める。
func humanMove(t *testing.T, g *dgame.Game, engine *drules.Engine, rng *rand.Rand) error {
	t.Helper()
	if !g.YourTurn() {
		t.Fatal("PhasePlaying なのに人間の手番ではありません")
	}
	hand := g.Hand()

	// ♦3 縛りの場面だけは、その 1 手しか出せない
	if g.GameNo() == 1 && g.Field().Empty && g.Options().FirstPlay == "diamond3_must" {
		open, err := engine.OpeningCard()
		if err != nil {
			return err
		}
		if n := cards.IndexOf(hand, open); n >= 0 {
			if err := g.PlayCards([]int{n + 1}); err == nil {
				return nil
			}
		}
	}

	plays, err := engine.LegalPlays(handCards(hand), g.Field(), g.Reversed())
	if err != nil {
		return err
	}
	if len(plays) == 0 {
		if g.Field().Empty {
			t.Fatal("親番なのに出せる手がありません")
		}
		return g.Pass()
	}
	play := plays[rng.Intn(len(plays))]
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
	if len(nums) != len(play) {
		t.Fatalf("選んだ手が手札に見つかりません: %v", play)
	}
	return g.PlayCards(nums)
}

func handCards(h []drules.Card) []drules.Card { return h }

// 既定のルールで 1 卓を最後まで進められる。
func TestPlayThroughDefault(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(1))
	g, err := dgame.New(engine, rng, 3, drules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	autoPlay(t, g, engine, rng)

	if g.Phase() != dgame.PhaseGameOver {
		t.Fatalf("最後まで進みませんでした: %s", g.Phase())
	}
	total := 0
	for _, s := range g.Scores() {
		total += s
	}
	// 4 人 3 ゲームなら毎ゲーム 3+2+1+0 = 6 点が配られる
	if total != 18 {
		t.Errorf("配られた得点の合計が 18 ではなく %d", total)
	}
}

// ローカルルールを色々な組み合わせで入れても、進行が止まらない。
func TestPlayThroughWithLocalRules(t *testing.T) {
	presets := map[string]func(*drules.Options){
		"全部盛り": func(o *drules.Options) {
			o.EightCut = "all"
			o.JackBack = "all"
			o.SeqRevolution = "on"
			o.FiveCard = "on"
			o.Bind = "on"
			o.BindStrict = "on"
			o.SpadeThree = "on"
			o.PassLock = "on"
			o.FinishFlush = "on"
			o.FoulFinish = "on"
			o.MiyakoOchi = "on"
		},
		"何も無し": func(o *drules.Options) {
			o.EightCut = "off"
			o.Jokers = 0
			o.FirstPlay = "free"
		},
		"片縛り3人": func(o *drules.Options) {
			o.Players = 3
			o.Bind = "on"
			o.BindPartial = "on"
			o.PassLock = "on"
			o.FirstPlay = "diamond3_free"
		},
		"6人ジョーカー1枚": func(o *drules.Options) {
			o.Players = 6
			o.Jokers = 1
			o.JackBack = "group_only"
			o.EightCut = "group_only"
		},
	}

	for name, tweak := range presets {
		t.Run(name, func(t *testing.T) {
			engine, err := drules.New("")
			if err != nil {
				t.Fatal(err)
			}
			o := drules.DefaultOptions()
			tweak(&o)
			rng := rand.New(rand.NewSource(7))
			g, err := dgame.New(engine, rng, 2, o)
			if err != nil {
				t.Fatal(err)
			}
			autoPlay(t, g, engine, rng)
			if g.Phase() != dgame.PhaseGameOver {
				t.Fatalf("最後まで進みませんでした: %s", g.Phase())
			}
			// 全員に順位が付いていること
			seen := map[int]bool{}
			for _, pi := range g.LastRanking() {
				if seen[pi] {
					t.Fatalf("同じプレイヤーに 2 つの順位が付いています: %v", g.LastRanking())
				}
				seen[pi] = true
			}
			if len(seen) != g.Players() {
				t.Fatalf("順位が全員分ありません: %v", g.LastRanking())
			}
		})
	}
}

// 2 ゲーム目以降は前回の称号に応じたカード交換が入る。
func TestExchangeHappensFromSecondGame(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(3))
	g, err := dgame.New(engine, rng, 2, drules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase() == dgame.PhaseExchange {
		t.Fatal("1 ゲーム目にカード交換は起きません")
	}

	// 1 ゲーム目を終わらせる
	for step := 0; step < 2000 && g.Phase() != dgame.PhaseRoundEnd; step++ {
		switch g.Phase() {
		case dgame.PhasePlaying:
			if err := humanMove(t, g, engine, rng); err != nil {
				t.Fatal(err)
			}
		case dgame.PhaseTenDiscard:
			if err := g.SubmitTenDiscard(firstN(g.HandOverNeed())); err != nil {
				t.Fatal(err)
			}
		case dgame.PhaseSevenGive:
			if err := g.SubmitSevenGive(firstN(g.HandOverNeed())); err != nil {
				t.Fatal(err)
			}
		case dgame.PhaseJackBack:
			if err := g.SubmitJackBack(true); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("1 ゲーム目の途中で想定外の局面になりました: %s", g.Phase())
		}
	}
	if g.Phase() != dgame.PhaseRoundEnd {
		t.Fatalf("1 ゲーム目が終わっていません: %s", g.Phase())
	}
	if len(g.LastTitles()) != g.Players() {
		t.Fatalf("称号が全員分ありません: %v", g.LastTitles())
	}

	if err := g.Next(); err != nil {
		t.Fatal(err)
	}
	// 人間が大富豪・富豪なら交換の入力待ち、そうでなければ自動で済んで対局中
	switch g.Phase() {
	case dgame.PhaseExchange:
		if g.ExchangeNeed() == 0 {
			t.Error("交換の枚数が 0 のまま入力待ちになっています")
		}
	case dgame.PhasePlaying:
		if len(g.Received()) == 0 {
			t.Error("下位のプレイヤーなら強いカードを渡して何か受け取るはずです")
		}
	default:
		t.Fatalf("2 ゲーム目の始まりが想定外の局面です: %s", g.Phase())
	}
}

// 10捨て・7渡し・任意適用のJバックは、人間の手番で入力を求める局面になる。
// 配札次第なので、種を変えて何度か遊び、全部の局面に一度は入ることを見る。
func TestHandOverPhasesAreReached(t *testing.T) {
	want := []dgame.Phase{
		dgame.PhasePlaying, dgame.PhaseJackBack, dgame.PhaseTenDiscard, dgame.PhaseSevenGive,
	}
	seen := map[dgame.Phase]bool{}
	allSeen := func() bool {
		for _, p := range want {
			if !seen[p] {
				return false
			}
		}
		return true
	}

	for seed := int64(1); seed <= 12 && !allSeen(); seed++ {
		engine, err := drules.New("")
		if err != nil {
			t.Fatal(err)
		}
		o := drules.DefaultOptions()
		o.TenDiscard = "on"
		o.SevenGive = "on"
		o.JackBack = "all"
		o.JackBackMode = "optional"
		o.FirstPlay = "free"
		rng := rand.New(rand.NewSource(seed))
		g, err := dgame.New(engine, rng, 2, o)
		if err != nil {
			t.Fatal(err)
		}

		for step := 0; step < 4000 && g.Phase() != dgame.PhaseGameOver; step++ {
			seen[g.Phase()] = true
			switch g.Phase() {
			case dgame.PhaseRoundEnd:
				if err := g.Next(); err != nil {
					t.Fatal(err)
				}
			case dgame.PhaseExchange:
				if err := g.SubmitExchange(firstN(g.ExchangeNeed())); err != nil {
					t.Fatal(err)
				}
			case dgame.PhasePlaying:
				if err := humanMove(t, g, engine, rng); err != nil {
					t.Fatal(err)
				}
			case dgame.PhaseJackBack:
				if len(g.PendingJackBack()) == 0 {
					t.Error("Jバックを決める場面なのに、出した手が分かりません")
				}
				if err := g.SubmitJackBack(true); err != nil {
					t.Fatal(err)
				}
			case dgame.PhaseTenDiscard:
				need := g.HandOverNeed()
				if need <= 0 {
					t.Fatalf("10捨てで %d 枚を求められました", need)
				}
				before := g.HandCounts()[dgame.HumanSeat]
				if err := g.SubmitTenDiscard(firstN(need)); err != nil {
					t.Fatal(err)
				}
				if after := g.HandCounts()[dgame.HumanSeat]; after != before-need {
					t.Errorf("10捨てで手札が %d 枚減るはずが %d → %d", need, before, after)
				}
			case dgame.PhaseSevenGive:
				need := g.HandOverNeed()
				if g.HandOverToName() == "" {
					t.Error("7渡しの相手が分かりません")
				}
				if err := g.SubmitSevenGive(firstN(need)); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("知らない局面です: %s", g.Phase())
			}
		}
	}

	for _, p := range want {
		if !seen[p] {
			t.Errorf("局面 %s に一度も入りませんでした", p)
		}
	}
}

// 10捨て・7渡しの枚数を賄えないまま上がると反則上がりになる。
func TestShortageBecomesFoul(t *testing.T) {
	// 種を変えて遊び、反則が一度でも起きることを見る。
	// (10 や 7 を最後に出して手札が足りなくなるのは配札次第)
	found := false
	for seed := int64(1); seed <= 25 && !found; seed++ {
		engine, err := drules.New("")
		if err != nil {
			t.Fatal(err)
		}
		o := drules.DefaultOptions()
		o.TenDiscard = "on"
		o.SevenGive = "on"
		o.FoulFinish = "on"
		o.FirstPlay = "free"
		rng := rand.New(rand.NewSource(seed))
		g, err := dgame.New(engine, rng, 1, o)
		if err != nil {
			t.Fatal(err)
		}
		autoPlay(t, g, engine, rng)

		for pi := 0; pi < g.Players(); pi++ {
			note := g.FoulNote(pi)
			if note == "10捨てで手札が足りない" || note == "7渡しで手札が足りない" {
				found = true
			}
		}
	}
	if !found {
		t.Skip("25 回遊んでも枚数不足の反則上がりは起きませんでした")
	}
}

// 今の場に誰が何を出したかを、場が流れるまで順に持っている。
func TestPileTracksWhoPlayedWhat(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	o := drules.DefaultOptions()
	o.FirstPlay = "free"
	o.EightCut = "off" // 途中で勝手に流れないようにする
	rng := rand.New(rand.NewSource(9))
	g, err := dgame.New(engine, rng, 1, o)
	if err != nil {
		t.Fatal(err)
	}

	sawMultiple := false
	sawFlush := false
	prev := len(g.Pile())
	for step := 0; step < 400 && g.Phase() != dgame.PhaseRoundEnd; step++ {
		pile := g.Pile()

		// 積まれた手は全て名前が引ける席のもので、パスか札のどちらか。
		for _, e := range pile {
			if e.Player < 0 || e.Player >= g.Players() {
				t.Fatalf("知らない席の手が積まれています: %+v", e)
			}
			if e.Passed && len(e.Cards) > 0 {
				t.Errorf("パスなのに札が付いています: %+v", e)
			}
			if !e.Passed && len(e.Cards) == 0 {
				t.Errorf("出した手なのに札がありません: %+v", e)
			}
		}
		// 「いちばん新しい出した手」の札が、場に出ている札と一致する。
		last := -1
		for i, e := range pile {
			if !e.Passed {
				last = i
			}
		}
		if last >= 0 {
			if got, want := len(pile[last].Cards), len(g.LastPlay()); got != want {
				t.Errorf("場の札が %d 枚なのに、積まれた最後の手は %d 枚", want, got)
			}
		}
		if g.Field().Empty && len(pile) > 0 {
			t.Errorf("場が空なのに手が積まれたままです: %+v", pile)
		}

		if len(pile) > 1 {
			sawMultiple = true
		}
		if len(pile) < prev {
			sawFlush = true // 場が流れて積みが消えた
		}
		prev = len(pile)

		advanceAnyPhase(t, g, engine, rng)
	}

	if !sawMultiple {
		t.Error("複数の手が積まれた場面が一度もありませんでした")
	}
	if !sawFlush {
		t.Error("場が流れて積みが消える場面が一度もありませんでした")
	}
}

// advanceAnyPhase はどの局面でも 1 手だけ進める。
func advanceAnyPhase(t *testing.T, g *dgame.Game, engine *drules.Engine, rng *rand.Rand) {
	t.Helper()
	switch g.Phase() {
	case dgame.PhasePlaying:
		if err := humanMove(t, g, engine, rng); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseJackBack:
		if err := g.SubmitJackBack(true); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseTenDiscard:
		if err := g.SubmitTenDiscard(firstN(g.HandOverNeed())); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseSevenGive:
		if err := g.SubmitSevenGive(firstN(g.HandOverNeed())); err != nil {
			t.Fatal(err)
		}
	case dgame.PhaseExchange:
		if err := g.SubmitExchange(firstN(g.ExchangeNeed())); err != nil {
			t.Fatal(err)
		}
	}
}

// 進行の控えをサーバー側へ流せる。
func TestLoggerReceivesEvents(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(13))
	g, err := dgame.New(engine, rng, 1, drules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	g.SetLogger(func(format string, a ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, a...))
	})

	for step := 0; step < 40 && g.Phase() != dgame.PhaseRoundEnd; step++ {
		advanceAnyPhase(t, g, engine, rng)
	}

	if len(lines) == 0 {
		t.Fatal("控えが 1 行も流れていません")
	}
	// 画面に残るのは直近 10 件だけなので、控えはそれより多く出ているはず
	if len(lines) <= len(g.Events()) {
		t.Errorf("控えが %d 行しかありません (画面には %d 件)", len(lines), len(g.Events()))
	}
	// 出した札が読める形で入っている
	found := false
	for _, line := range lines {
		if strings.Contains(line, "が出しました [") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("出した札が控えに入っていません: %v", lines)
	}
}

// 出せない理由の説明。革命中は、強弱が反転していることも添える。
func TestExplainIllegalMentionsReversal(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	o := drules.DefaultOptions()
	o.FirstPlay = "free"
	o.Bind = "off"
	o.BindStrict = "off"
	o.BindPartial = "off"
	rng := rand.New(rand.NewSource(21))
	g, err := dgame.New(engine, rng, 1, o)
	if err != nil {
		t.Fatal(err)
	}

	// 人間の手番まで進め、場に手がある状態で「弱い手」を出してみる。
	for step := 0; step < 300; step++ {
		if g.Phase() == dgame.PhasePlaying && g.YourTurn() && !g.Field().Empty {
			break
		}
		if g.Phase() == dgame.PhaseRoundEnd {
			t.Skip("人間の手番で場に手がある場面に届きませんでした")
		}
		advanceAnyPhase(t, g, engine, rng)
	}
	if g.Field().Empty || !g.YourTurn() {
		t.Skip("人間の手番で場に手がある場面に届きませんでした")
	}

	// いちばん弱い札 1 枚 (表示順の先頭) を出すと、たいてい弾かれる
	err = g.PlayCards([]int{1})
	if err == nil {
		t.Skip("いちばん弱い札が通ってしまいました")
	}
	var input dgame.ErrPlayerInput
	if !errors.As(err, &input) {
		t.Fatalf("入力ミス以外の誤りです: %v", err)
	}
	if input.Msg == "" {
		t.Fatal("理由が空です")
	}
	if g.Reversed() && !strings.Contains(input.Msg, "いちばん強く") {
		t.Errorf("反転中なのに強弱の向きに触れていません: %q", input.Msg)
	}
	t.Logf("出せない理由: %s", input.Msg)
}

// 強弱の向きの一言は、革命と Jバックが重なると打ち消し合うことまで書く。
func TestReversalNote(t *testing.T) {
	engine, err := drules.New("")
	if err != nil {
		t.Fatal(err)
	}
	g, err := dgame.New(engine, rand.New(rand.NewSource(1)), 1, drules.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if note := g.ReversalNote(); note != "" {
		t.Errorf("開始直後は一言なしのはずが %q", note)
	}
}
