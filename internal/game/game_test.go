package game

// game_test.go — 状態機械の検証。
// 人間の席も「合法手の先頭を選ぶ」だけの自動操作で 1 ゲーム通す。

import (
	"math/rand"
	"testing"

	"hearts/internal/rules"
)

func newTestGame(t *testing.T, seed int64, limit int) *Game {
	t.Helper()
	return newTestGameOpts(t, seed, limit, rules.DefaultOptions())
}

func newTestGameOpts(t *testing.T, seed int64, limit int, opts rules.Options) *Game {
	t.Helper()
	engine, err := rules.New("")
	if err != nil {
		t.Fatalf("Prolog の読み込みに失敗: %v", err)
	}
	g, err := New(engine, rand.New(rand.NewSource(seed)), limit, opts)
	if err != nil {
		t.Fatalf("ゲームを開始できません: %v", err)
	}
	return g
}

// handNumber は表示順のどこにそのカードがあるかを返す (1 始まり)。
func handNumber(t *testing.T, g *Game, card rules.Card) int {
	t.Helper()
	for i, c := range g.Hand() {
		if c == card {
			return i + 1
		}
	}
	t.Fatalf("%v が手札にありません", card)
	return 0
}

// autoPlay は人間の席を自動で 1 手進める。
func autoStep(t *testing.T, g *Game) {
	t.Helper()
	switch g.Phase() {
	case PhasePassing:
		nums := make([]int, g.PassCount())
		for i := range nums {
			nums[i] = i + 1
		}
		if err := g.SubmitPass(nums); err != nil {
			t.Fatalf("カード交換に失敗: %v", err)
		}
	case PhasePlaying:
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
	case PhaseTrickDone, PhaseRoundDone:
		if err := g.Next(); err != nil {
			t.Fatalf("進行に失敗: %v", err)
		}
	}
}

// どの得点の数え方でもゲームが最後まで進み、スラムの精算が流儀どおりになる。
func TestScoringRulesReachGameOver(t *testing.T) {
	for _, mode := range []rules.ScoringRule{rules.ScoringSlamOthers, rules.ScoringSlamZero, rules.ScoringLife} {
		t.Run(string(mode), func(t *testing.T) {
			opts := rules.DefaultOptions()
			opts.Scoring = mode
			g := newTestGameOpts(t, 20260811, 50, opts)

			for step := 0; g.Phase() != PhaseGameOver; step++ {
				if step > 20000 {
					t.Fatal("ゲームが終わりません")
				}
				if g.Phase() == PhaseRoundDone && g.RoundSlammer() >= 0 {
					sum := 0
					for _, s := range g.RoundFinal() {
						sum += s
					}
					want := map[rules.ScoringRule]int{
						rules.ScoringSlamOthers: g.MaxPoints() * (g.Players() - 1),
						rules.ScoringSlamZero:   0,
						rules.ScoringLife:       -g.MaxPoints(),
					}[mode]
					if sum != want {
						t.Fatalf("スラムの失点合計 = %d; want %d", sum, want)
					}
				}
				autoStep(t, g)
			}

			// 体力制でも、持ち点 0 以下 = 累計失点が開始点以上。
			over := false
			for _, s := range g.Scores() {
				if s >= g.Limit() {
					over = true
				}
			}
			if !over {
				t.Fatalf("終了したのに誰も上限に達していません: %v", g.Scores())
			}
		})
	}
}

// 1 ゲームを最後まで通し、局面が正しく遷移して終了することを確かめる。
func TestFullGameReachesGameOver(t *testing.T) {
	g := newTestGame(t, 20260811, 50)

	rounds := 0
	for step := 0; g.Phase() != PhaseGameOver; step++ {
		if step > 20000 {
			t.Fatal("ゲームが終わりません")
		}

		// ラウンド精算のたびに失点の合計を確かめる。
		if g.Phase() == PhaseRoundDone {
			rounds++
			sum := 0
			for _, s := range g.RoundFinal() {
				sum += s
			}
			want := g.MaxPoints()
			if g.RoundSlammer() >= 0 {
				want = g.MaxPoints() * (g.Players() - 1)
			}
			if sum != want {
				t.Fatalf("第%dラウンドの失点合計 = %d; want %d (slammer=%d)",
					g.Round(), sum, want, g.RoundSlammer())
			}
		}
		autoStep(t, g)
	}

	if rounds == 0 {
		t.Fatal("1 ラウンドも精算されていません")
	}

	over, err := g.engine.GameOver(g.Scores(), g.Limit())
	if err != nil {
		t.Fatal(err)
	}
	if !over {
		t.Fatalf("終了したのに誰も上限に達していません: %v", g.Scores())
	}

	// 勝者は最少失点
	w := g.Winner()
	for i, s := range g.Scores() {
		if s < g.Scores()[w] {
			t.Fatalf("勝者 %d (%d点) より %d (%d点) の方が少ない", w, g.Scores()[w], i, s)
		}
	}
}

// 「開始カードから」の流儀では、第1トリックは必ず開始カードから始まり、
// リードした人が先手になる。
func TestFirstTrickStartsWithOpeningCard(t *testing.T) {
	opts := rules.DefaultOptions()
	opts.FirstTrickLead = rules.LeadOpeningCard
	g := newTestGameOpts(t, 7, 100, opts)

	// 交換を済ませてトリックプレイに入る
	for g.Phase() == PhasePassing {
		autoStep(t, g)
	}

	opening, err := g.engine.OpeningCard()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Played()) == 0 {
		// 人間が先手なら、出せるのは開始カードだけ
		legal, err := g.LegalCards()
		if err != nil {
			t.Fatal(err)
		}
		if len(legal) != 1 || legal[0] != opening {
			t.Fatalf("先手の合法手 = %v; 開始カード %v だけのはず", legal, opening)
		}
		return
	}
	if g.Played()[0] != opening {
		t.Fatalf("第1トリックの最初のカード = %v; want %v", g.Played()[0], opening)
	}
}

// 手番でないときや不正な番号を渡したときは ErrPlayerInput になる (500 にしない)。
func TestInvalidInputIsPlayerError(t *testing.T) {
	g := newTestGame(t, 3, 100)

	// 交換中にカードを出そうとする
	if err := g.PlayCard(1); err == nil {
		t.Fatal("交換中に PlayCard が通ってしまいました")
	} else if _, ok := err.(ErrPlayerInput); !ok {
		t.Fatalf("err = %T; ErrPlayerInput のはず", err)
	}

	// 枚数が足りない交換
	if err := g.SubmitPass([]int{1, 2}); err == nil {
		t.Fatal("2 枚の交換が通ってしまいました")
	} else if _, ok := err.(ErrPlayerInput); !ok {
		t.Fatalf("err = %T; ErrPlayerInput のはず", err)
	}

	// 範囲外の番号
	if err := g.SubmitPass([]int{1, 2, 999}); err == nil {
		t.Fatal("範囲外の番号が通ってしまいました")
	}

	// 同じ番号の重複
	if err := g.SubmitPass([]int{1, 1, 2}); err == nil {
		t.Fatal("重複した番号が通ってしまいました")
	}
}

// ♠Q が場に出ただけでブレイクし、以降ハートでリードできるようになる。
func TestQueenOfSpadesBreaksHearts(t *testing.T) {
	opts := rules.DefaultOptions()
	opts.HeartsBreak = rules.HeartsBreakOn
	opts.FirstTrickPoints = rules.AllowPoints // ♠Q が第1トリックで出てもよいようにする

	// ♥ を一度も出さずに ♠Q が出るトリックを探す
	for seed := int64(1); seed <= 40; seed++ {
		g := newTestGameOpts(t, seed, 100, opts)
		for step := 0; step < 400 && g.Phase() != PhaseRoundDone; step++ {
			before := g.HeartsBroken()
			autoStep(t, g)
			if before || !g.HeartsBroken() {
				continue
			}
			// このトリックでブレイクした。ハートが無く ♠Q だけなら狙いどおり。
			hasHeart := false
			hasQueen := false
			for _, card := range g.Played() {
				if card.Suit == "h" {
					hasHeart = true
				}
				if card == (rules.Card{Suit: "s", Rank: "q"}) {
					hasQueen = true
				}
			}
			if hasQueen && !hasHeart {
				return // 期待どおりの挙動を確認できた
			}
			break // このラウンドはハートでブレイクしたので次のシードへ
		}
	}
	t.Skip("40 シード試したが ♠Q だけでブレイクする局面が出なかった")
}

// 交換をすると手札の枚数が戻り、渡したカードは手元から消える。
func TestPassExchangesCards(t *testing.T) {
	g := newTestGame(t, 11, 100)
	if g.Phase() != PhasePassing {
		t.Skip("このシードでは交換なしのラウンドから始まる")
	}

	before := append([]rules.Card(nil), g.Hand()...)
	size := len(before)
	given := []rules.Card{before[0], before[1], before[2]}

	if err := g.SubmitPass([]int{1, 2, 3}); err != nil {
		t.Fatal(err)
	}

	if len(g.Hand()) != size {
		t.Fatalf("交換後の手札 = %d 枚; want %d 枚", len(g.Hand()), size)
	}
	for _, c := range given {
		for _, h := range g.Hand() {
			if h == c {
				t.Fatalf("渡したはずの %v が手札に残っています", c)
			}
		}
	}
	if len(g.Received()) != g.PassCount() {
		t.Fatalf("受け取った枚数 = %d; want %d", len(g.Received()), g.PassCount())
	}
}
