// Package game はハート 1 局分の進行を受け持つ。
//
// 画面や HTTP のことは一切知らない。ルール判定も自分では書かず、すべて
// rules.Engine 経由で Prolog (hearts.pl とその上のレイヤ) に尋ねる。
// ここにあるのは「今どの局面か」「次に何が起きるか」の管理だけ。
package game

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"hearts/internal/rules"
)

// Phase は今プレイヤーに何を求めている局面か。
type Phase string

const (
	PhasePassing   Phase = "passing"    // カード交換の選択待ち
	PhasePlaying   Phase = "playing"    // 出すカードの選択待ち
	PhaseTrickDone Phase = "trick_done" // トリック終了、次へ進む待ち
	PhaseRoundDone Phase = "round_done" // ラウンド終了、次へ進む待ち
	PhaseGameOver  Phase = "game_over"  // ゲーム終了
)

// HumanSeat は人間のプレイヤー番号。残りは CPU。
const HumanSeat = 0

// ErrPlayerInput は「入力がルール上おかしい」ことを表す。
// サーバーはこれを 500 ではなく画面上のメッセージとして扱う。
type ErrPlayerInput struct{ Msg string }

func (e ErrPlayerInput) Error() string { return e.Msg }

func inputErr(format string, a ...interface{}) error {
	return ErrPlayerInput{Msg: fmt.Sprintf(format, a...)}
}

// Game は 1 ゲーム分の状態。並行安全ではない。
type Game struct {
	engine *rules.Engine
	rng    *rand.Rand
	opts   rules.Options

	players   int
	handSize  int
	passCount int
	maxPoints int // 1 ラウンドの失点総和 (通常 26)
	limit     int // この失点に達したら終了

	names  []string
	scores []int

	round    int
	phase    Phase
	hands    [][]rules.Card
	taken    [][]rules.Card
	passDir  string
	received []rules.Card // 人間が交換で受け取ったカード

	trickNo  int
	leader   int
	turn     int
	played   []rules.Card
	playedBy []int
	lead     *rules.Card
	broken   bool

	// 直前のトリックの結果 (PhaseTrickDone のときに使う)
	trickWinner int
	trickPoints int

	// 直前のラウンドの結果 (PhaseRoundDone のときに使う)
	roundFinal   []int
	roundSlammer int

	notice string // ハートブレイクなどの一時メッセージ
}

// New は選んだ流儀を Prolog に反映し、人数・手札枚数などを取得して
// ゲームを開始する。engine は 1 局につき 1 つ専有すること。
func New(engine *rules.Engine, rng *rand.Rand, limit int, opts rules.Options) (*Game, error) {
	if err := opts.Apply(engine); err != nil {
		return nil, err
	}

	players, err := engine.TotalPlayers()
	if err != nil {
		return nil, err
	}
	handSize, err := engine.HandSize()
	if err != nil {
		return nil, err
	}
	passCount, err := engine.PassCount()
	if err != nil {
		return nil, err
	}
	maxPoints, err := engine.TotalRoundPoints()
	if err != nil {
		return nil, err
	}

	names := make([]string, players)
	names[HumanSeat] = "あなた"
	for i := 1; i < players; i++ {
		names[i] = fmt.Sprintf("CPU%d", i)
	}

	g := &Game{
		engine:       engine,
		rng:          rng,
		opts:         opts,
		players:      players,
		handSize:     handSize,
		passCount:    passCount,
		maxPoints:    maxPoints,
		limit:        limit,
		names:        names,
		scores:       make([]int, players),
		round:        1,
		roundSlammer: -1,
	}
	if err := g.startRound(); err != nil {
		return nil, err
	}
	return g, nil
}

// =============================================
// 参照 (画面を組み立てる側から読む)
// =============================================

func (g *Game) Phase() Phase           { return g.phase }
func (g *Game) Round() int             { return g.round }
func (g *Game) Names() []string        { return g.names }
func (g *Game) Scores() []int          { return g.scores }
func (g *Game) Limit() int             { return g.limit }
func (g *Game) MaxPoints() int         { return g.maxPoints }
func (g *Game) PassCount() int         { return g.passCount }
func (g *Game) PassDir() string        { return g.passDir }
func (g *Game) Received() []rules.Card { return g.received }
func (g *Game) TrickNo() int           { return g.trickNo }
func (g *Game) TrickTotal() int        { return g.handSize }
func (g *Game) Played() []rules.Card   { return g.played }
func (g *Game) PlayedBy() []int        { return g.playedBy }
func (g *Game) Lead() *rules.Card      { return g.lead }
func (g *Game) HeartsBroken() bool     { return g.broken }
func (g *Game) TrickWinner() int       { return g.trickWinner }
func (g *Game) TrickPoints() int       { return g.trickPoints }
func (g *Game) RoundFinal() []int      { return g.roundFinal }
func (g *Game) RoundSlammer() int      { return g.roundSlammer }
func (g *Game) Notice() string         { return g.notice }
func (g *Game) Players() int           { return g.players }
func (g *Game) Options() rules.Options { return g.opts }

// Turn は今カードを出す番のプレイヤー番号。
func (g *Game) Turn() int { return g.turn }

// YourTurn は人間の入力を待っているか。
func (g *Game) YourTurn() bool {
	return g.phase == PhasePlaying && g.turn == HumanSeat
}

// Hand は人間の手札を表示順 (♠♥♦♣、各スート内は強い順) で返す。
// 画面の番号はこの並びの 1 始まりの位置。
func (g *Game) Hand() []rules.Card {
	return g.sortForDisplay(g.hands[HumanSeat])
}

// LegalCards は今この瞬間に人間が出せるカード。
func (g *Game) LegalCards() ([]rules.Card, error) {
	if !g.YourTurn() {
		return nil, nil
	}
	return g.engine.LegalCards(g.hands[HumanSeat], g.lead, g.trickNo, g.broken)
}

// TakenPoints は各プレイヤーがこのラウンドで取った失点。
func (g *Game) TakenPoints() ([]int, error) {
	out := make([]int, g.players)
	for i := range out {
		p, err := g.engine.CalculateScore(g.taken[i])
		if err != nil {
			return nil, err
		}
		out[i] = p
	}
	return out, nil
}

// TakenCounts は各プレイヤーがこのラウンドで取った枚数。
func (g *Game) TakenCounts() []int {
	out := make([]int, g.players)
	for i := range out {
		out[i] = len(g.taken[i])
	}
	return out
}

// Winner は最少失点のプレイヤー。
func (g *Game) Winner() int {
	best := 0
	for i, s := range g.scores {
		if s < g.scores[best] {
			best = i
		}
	}
	return best
}

// sortForDisplay は ♠♥♦♣ の順、各スート内はランクの強い順に並べる。
// 強さは hearts.pl の rank_value/2 に由来する。
func (g *Game) sortForDisplay(cards []rules.Card) []rules.Card {
	out := append([]rules.Card(nil), cards...)
	suitRank := map[string]int{"s": 0, "h": 1, "d": 2, "c": 3}
	sort.SliceStable(out, func(i, j int) bool {
		if suitRank[out[i].Suit] != suitRank[out[j].Suit] {
			return suitRank[out[i].Suit] < suitRank[out[j].Suit]
		}
		return g.engine.RankValue(out[i].Rank) > g.engine.RankValue(out[j].Rank)
	})
	return out
}

// =============================================
// プレイヤーからの操作
// =============================================

// SubmitPass は人間が選んだ交換カード (手札の表示番号) を受け取る。
func (g *Game) SubmitPass(numbers []int) error {
	if g.phase != PhasePassing {
		return inputErr("今はカードを交換する場面ではありません。")
	}
	shown := g.Hand()
	if len(numbers) != g.passCount {
		return inputErr("渡すカードを %d 枚選んでください。", g.passCount)
	}
	seen := map[int]bool{}
	picked := make([]rules.Card, 0, len(numbers))
	for _, n := range numbers {
		if n < 1 || n > len(shown) || seen[n] {
			return inputErr("カードの選び方が正しくありません。")
		}
		seen[n] = true
		picked = append(picked, shown[n-1])
	}

	selected := make([][]rules.Card, g.players)
	selected[HumanSeat] = picked
	for i := 0; i < g.players; i++ {
		if i == HumanSeat {
			continue
		}
		cards, err := g.engine.ChoosePass(g.hands[i], g.passCount)
		if err != nil {
			return err
		}
		selected[i] = cards
	}

	// 全員同時に渡す
	for i := 0; i < g.players; i++ {
		for _, c := range selected[i] {
			g.hands[i] = rules.Remove(g.hands[i], c)
		}
	}
	g.received = nil
	for i := 0; i < g.players; i++ {
		to, err := g.engine.PassTarget(i, g.passDir, g.players)
		if err != nil {
			return err
		}
		g.hands[to] = append(g.hands[to], selected[i]...)
		if to == HumanSeat {
			g.received = append(g.received, selected[i]...)
		}
	}
	g.received = g.sortForDisplay(g.received)

	return g.startTricks()
}

// PlayCard は人間が選んだカード (手札の表示番号) を場に出す。
func (g *Game) PlayCard(number int) error {
	if g.phase != PhasePlaying {
		return inputErr("今はカードを出す場面ではありません。")
	}
	if g.turn != HumanSeat {
		return inputErr("今はあなたの手番ではありません。")
	}
	shown := g.Hand()
	if number < 1 || number > len(shown) {
		return inputErr("そのカードは手札にありません。")
	}
	card := shown[number-1]

	ok, err := g.engine.IsLegal(g.hands[HumanSeat], g.lead, g.trickNo, g.broken, card)
	if err != nil {
		return err
	}
	if !ok {
		reason, err := g.explainIllegal(card)
		if err != nil {
			return err
		}
		return ErrPlayerInput{Msg: reason}
	}

	g.notice = ""
	if err := g.commit(HumanSeat, card); err != nil {
		return err
	}
	return g.advance()
}

// Next はトリック終了・ラウンド終了の画面から先へ進める。
func (g *Game) Next() error {
	switch g.phase {
	case PhaseTrickDone:
		if g.trickNo < g.handSize {
			g.trickNo++
			g.leader = g.trickWinner
			g.turn = g.leader
			g.played = nil
			g.playedBy = nil
			g.lead = nil
			g.notice = ""
			g.phase = PhasePlaying
			return g.advance()
		}
		return g.finishRound()

	case PhaseRoundDone:
		over, err := g.engine.GameOver(g.scores, g.limit)
		if err != nil {
			return err
		}
		if over {
			g.phase = PhaseGameOver
			return nil
		}
		g.round++
		return g.startRound()
	}
	return nil
}

// =============================================
// 内部の進行
// =============================================

func (g *Game) startRound() error {
	hands, err := g.deal()
	if err != nil {
		return err
	}
	g.hands = hands
	g.taken = make([][]rules.Card, g.players)
	g.broken = false
	g.notice = ""
	g.received = nil
	g.roundFinal = nil
	g.roundSlammer = -1
	g.played = nil
	g.playedBy = nil
	g.lead = nil
	g.trickNo = 1

	dir, err := g.engine.PassDirection(g.round)
	if err != nil {
		return err
	}
	g.passDir = dir

	if dir == "hold" {
		return g.startTricks()
	}
	g.phase = PhasePassing
	return nil
}

// deal はデッキを Prolog から取得してシャッフルし、配る。
// 配り終えた状態は hearts.pl の validate_deal/1 で検証する。
func (g *Game) deal() ([][]rules.Card, error) {
	deck, err := g.engine.Deck()
	if err != nil {
		return nil, err
	}
	g.rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	hands := make([][]rules.Card, g.players)
	for i := 0; i < g.players; i++ {
		hands[i] = append([]rules.Card(nil), deck[i*g.handSize:(i+1)*g.handSize]...)
	}

	ok, err := g.engine.ValidateDeal(hands)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("配札が hearts.pl の validate_deal/1 を通りませんでした")
	}
	return hands, nil
}

// startTricks は開始カード (♣2) の持ち主を先手にしてトリックプレイに入る。
func (g *Game) startTricks() error {
	opening, err := g.engine.OpeningCard()
	if err != nil {
		return err
	}
	leader := -1
	for i, hand := range g.hands {
		for _, c := range hand {
			if c == opening {
				leader = i
			}
		}
	}
	if leader < 0 {
		return fmt.Errorf("開始カード %s が誰の手札にもありません", rules.CardLabel(opening))
	}

	g.leader = leader
	g.turn = leader
	g.trickNo = 1
	g.played = nil
	g.playedBy = nil
	g.lead = nil
	g.phase = PhasePlaying
	return g.advance()
}

// advance は人間の手番になるかトリックが揃うまで CPU を進める。
func (g *Game) advance() error {
	if g.phase != PhasePlaying {
		return nil
	}
	for len(g.played) < g.players && g.turn != HumanSeat {
		if err := g.cpuPlay(g.turn); err != nil {
			return err
		}
	}
	if len(g.played) == g.players {
		return g.resolveTrick()
	}
	return nil
}

func (g *Game) cpuPlay(pi int) error {
	card, err := g.engine.ChooseCard(g.hands[pi], g.lead, g.played, g.trickNo, g.broken)
	if err != nil {
		return err
	}
	return g.commit(pi, card)
}

// commit は 1 枚を場に出す。人間の入力も CPU の出力も、最後に必ず
// hearts.pl の規則を通るかどうかを確かめてから反映する。
func (g *Game) commit(pi int, card rules.Card) error {
	ok, err := g.engine.IsLegal(g.hands[pi], g.lead, g.trickNo, g.broken, card)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s が出そうとした %s は hearts.pl のルールに反します",
			g.names[pi], rules.CardLabel(card))
	}

	g.hands[pi] = rules.Remove(g.hands[pi], card)
	g.played = append(g.played, card)
	g.playedBy = append(g.playedBy, pi)
	if len(g.played) == 1 {
		c := card
		g.lead = &c
	}
	g.turn = (pi + 1) % g.players
	return nil
}

func (g *Game) resolveTrick() error {
	// 勝者判定は hearts.pl の determine_winner/3
	win, err := g.engine.DetermineWinner(g.played, g.lead.Suit)
	if err != nil {
		return err
	}
	i := rules.IndexOf(g.played, win)
	if i < 0 {
		return fmt.Errorf("勝ち札 %s が場にありません", rules.CardLabel(win))
	}
	winner := g.playedBy[i]
	g.taken[winner] = append(g.taken[winner], g.played...)
	g.trickWinner = winner

	g.trickPoints, err = g.engine.CalculateScore(g.played)
	if err != nil {
		return err
	}

	if !g.broken {
		g.broken, err = g.engine.ContainsPointCard(g.played)
		if err != nil {
			return err
		}
		// ブレイク規則を使わない設定では、知らせても意味が無い。
		if g.broken && g.opts.UsesHeartsBreak() {
			g.notice = "ハートブレイク！ 得点札が出たので、以降はハートでリードできます。"
		}
	}

	g.phase = PhaseTrickDone
	return nil
}

func (g *Game) finishRound() error {
	// 精算は hearts.pl の calculate_score/2 + スラム判定
	final, slammer, err := g.engine.RoundScores(g.taken)
	if err != nil {
		return err
	}
	for i := range g.scores {
		g.scores[i] += final[i]
	}
	g.roundFinal = final
	g.roundSlammer = slammer
	g.phase = PhaseRoundDone
	return nil
}

// explainIllegal は出せない理由を日本語で説明する。
// 判定そのものは Prolog が済ませているので、ここは案内文だけを組み立てる。
func (g *Game) explainIllegal(card rules.Card) (string, error) {
	if g.trickNo == 1 && g.lead == nil && g.opts.FirstTrickLead == rules.LeadOpeningCard {
		opening, err := g.engine.OpeningCard()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("第1トリックは %s から始めます。", rules.CardLabel(opening)), nil
	}

	if g.lead != nil && card.Suit != g.lead.Suit {
		for _, c := range g.hands[HumanSeat] {
			if c.Suit == g.lead.Suit {
				return fmt.Sprintf("台札は%sです。%sを持っているので必ず出してください。",
					rules.SuitName(g.lead.Suit), rules.SuitName(g.lead.Suit)), nil
			}
		}
	}

	if g.trickNo == 1 {
		switch g.opts.FirstTrickPoints {
		case rules.ForbidAllPoints:
			p, err := g.engine.CardPoint(card)
			if err != nil {
				return "", err
			}
			if p > 0 {
				return "第1トリックでは点数札 (ハートとスペードのQ) を出せません。", nil
			}
		case rules.ForbidHearts:
			if card.Suit == "h" {
				return "第1トリックではハートを出せません。", nil
			}
		}
	}

	if g.lead == nil && card.Suit == "h" && !g.broken && g.opts.UsesHeartsBreak() {
		return "まだ得点札が場に出ていないので、ハートからはリードできません。", nil
	}

	return "そのカードはルール上出せません。", nil
}
