// Package daifugo は大富豪 1 卓分の進行を受け持つ。
//
// internal/game (ハート) と同じ考え方で、画面や HTTP のことは一切知らず、
// ルール判定も自分では書かない。「出せるか」「革命になるか」「反則上がりか」
// といった判断は全て rules/daifugo.Engine 経由で Prolog に尋ね、
// ここにあるのは「今どの局面か」「次に誰の番か」の管理だけ。
package daifugo

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"hearts/internal/cards"
	"hearts/internal/playerr"
	"hearts/internal/rules/daifugo"
)

// Phase は今プレイヤーに何を求めている局面か。
type Phase string

const (
	PhaseExchange   Phase = "exchange"    // カード交換の選択待ち
	PhasePlaying    Phase = "playing"     // 出すカード (またはパス) の選択待ち
	PhaseJackBack   Phase = "jack_back"   // Jバックを使うかどうかの選択待ち
	PhaseTenDiscard Phase = "ten_discard" // 10捨てで捨てるカードの選択待ち
	PhaseSevenGive  Phase = "seven_give"  // 7渡しで渡すカードの選択待ち
	PhaseRoundEnd   Phase = "round_end"   // 1 ゲーム終了、順位の確認待ち
	PhaseGameOver   Phase = "game_over"   // 全ゲーム終了
)

// HumanSeat は人間のプレイヤー番号。残りは CPU。
const HumanSeat = 0

// 称号の日本語表記。Prolog 側は英字のアトムで返す。
var titleJa = map[string]string{
	"daifugo":   "大富豪",
	"fugo":      "富豪",
	"heimin":    "平民",
	"hinmin":    "貧民",
	"daihinmin": "大貧民",
}

// TitleJa は称号の日本語表記を返す。
func TitleJa(title string) string {
	if t, ok := titleJa[title]; ok {
		return t
	}
	return title
}

// ErrPlayerInput は「入力がルール上おかしい」ことを表す。
// サーバーはこれを 500 ではなく画面上のメッセージとして扱う。
// 3 つのゲームで同じ型を使う (internal/playerr)。
type ErrPlayerInput = playerr.Error

func inputErr(format string, a ...interface{}) error {
	return playerr.Errorf(format, a...)
}

// Event は場で起きたことの記録。画面に履歴として出す。
type Event struct {
	Player int // -1 なら場全体の出来事
	Text   string
	Cards  []daifugo.Card
}

// PileEntry は「今の場」に積まれた 1 手。場が流れると消える。
//
// 大富豪はトリックの区切りが無く、1 回の応答で CPU が何人も続けて手を出す。
// 誰が何を出したのかを追えるように、場が流れるまでの手を順に持っておく。
type PileEntry struct {
	Player int
	Cards  []daifugo.Card
	Passed bool
}

// maxEvents は画面に残す履歴の数。
const maxEvents = 10

// pendingPlay は「1 手を出したあとの処理」がどこまで進んだかの覚え書き。
// Jバック・10捨て・7渡しで人間の入力を待つあいだだけ意味を持つ。
type pendingPlay struct {
	seat     int
	play     []daifugo.Card
	cut      bool // 8流し・スペ3返しで場が流れるか
	jackBack bool // Jバックの引き金になる手か
}

// Game は複数ゲーム通しの 1 卓分の状態。並行安全ではない。
type Game struct {
	engine *daifugo.Engine
	rng    *rand.Rand
	opts   daifugo.Options

	players    int
	totalGames int

	names      []string
	scores     []int
	titles     []string // 前のゲームの称号 (プレイヤー番号順)。初回は空
	rankPoints []int    // 順位ごとの得点

	gameNo int
	phase  Phase

	hands [][]daifugo.Card
	deck  []daifugo.Card

	// 場の状態
	field      daifugo.Field
	lastPlay   []daifugo.Card // 場に出ている実際のカード (表示用)
	pile       []PileEntry    // 今の場に出た手を順に。流れると空になる
	revolution bool           // フォーカードなどによる革命
	jackBack   bool           // Jバック (場が流れると解除)
	lastPlayer int            // 最後にカードを出した人。-1 なら今の場にはまだ誰も出していない
	anyPlayed  bool           // このゲームで 1 枚でもカードが出たか (♦3 縛りの判定に使う)

	turn        int
	passedSince []bool // 最後に誰かが出してからパスしたか
	locked      []bool // パスしたら流れるまで出せない設定でのロック
	out         []bool // 上がった (または都落ちで抜けた) プレイヤー

	finished  []int // 上がった順 (反則上がり・都落ちを除く)
	penalized []int // 反則上がり・都落ちの発生順。下位から順に割り当てる
	foulNote  map[int]string

	// 10捨て・7渡しで手札が足りなかった人。上がった時点で反則にする。
	foulPending map[int]string

	// 出したあとの処理 (Jバック・10捨て・7渡し) の途中で人間の入力を
	// 待つあいだ、どこまで進んだかを覚えておく。
	pending      pendingPlay
	handOverNeed int // 捨てる・渡す枚数
	handOverTo   int // 7渡しの相手
	discarded    []daifugo.Card

	// カード交換
	exchangeNeed int              // 人間が選ぶ枚数。0 なら人間の選択は不要
	exchangeTo   int              // 人間が渡す相手
	received     []daifugo.Card   // 人間が受け取ったカード
	pendingGive  [][]daifugo.Card // 交換で渡すカード (プレイヤー番号順)

	// 直前のゲームの結果
	lastRanking []int // 順位順のプレイヤー番号
	lastTitles  []string

	prevDaifugo int // 前のゲームの大富豪。都落ちの判定に使う。-1 なら無し

	events []Event
	notice string

	// logf は進行の控えを外へ流す先。nil なら何もしない。
	// 画面に残らない古い出来事をあとから追うために、サーバーのログへ繋ぐ。
	logf func(format string, a ...interface{})
}

// New は選んだローカルルールを Prolog に反映してゲームを始める。
// engine は 1 卓につき 1 つ専有すること。
func New(engine *daifugo.Engine, rng *rand.Rand, totalGames int, opts daifugo.Options) (*Game, error) {
	if err := opts.Apply(engine); err != nil {
		return nil, err
	}
	players, err := engine.Players()
	if err != nil {
		return nil, err
	}
	points, err := engine.RankPoints(players)
	if err != nil {
		return nil, err
	}
	deck, err := engine.Deck()
	if err != nil {
		return nil, err
	}

	names := make([]string, players)
	names[HumanSeat] = "あなた"
	for i := 1; i < players; i++ {
		names[i] = fmt.Sprintf("CPU%d", i)
	}

	g := &Game{
		engine:      engine,
		rng:         rng,
		opts:        opts,
		players:     players,
		totalGames:  totalGames,
		names:       names,
		scores:      make([]int, players),
		rankPoints:  points,
		deck:        deck,
		gameNo:      1,
		prevDaifugo: -1,
		foulNote:    map[int]string{},
		foulPending: map[int]string{},
		handOverTo:  -1,
	}
	if err := g.startGame(); err != nil {
		return nil, err
	}
	return g, nil
}

// =============================================
// 参照 (画面を組み立てる側から読む)
// =============================================

func (g *Game) Phase() Phase             { return g.phase }
func (g *Game) GameNo() int              { return g.gameNo }
func (g *Game) TotalGames() int          { return g.totalGames }
func (g *Game) Players() int             { return g.players }
func (g *Game) Names() []string          { return g.names }
func (g *Game) Scores() []int            { return g.scores }
func (g *Game) Titles() []string         { return g.titles }
func (g *Game) Field() daifugo.Field     { return g.field }
func (g *Game) LastPlay() []daifugo.Card { return g.lastPlay }
func (g *Game) Revolution() bool         { return g.revolution }
func (g *Game) JackBack() bool           { return g.jackBack }
func (g *Game) Turn() int                { return g.turn }
func (g *Game) Out() []bool              { return g.out }
func (g *Game) Events() []Event          { return g.events }
func (g *Game) Notice() string           { return g.notice }
func (g *Game) Options() daifugo.Options { return g.opts }
func (g *Game) LastRanking() []int       { return g.lastRanking }
func (g *Game) LastTitles() []string     { return g.lastTitles }
func (g *Game) Received() []daifugo.Card { return g.received }
func (g *Game) ExchangeNeed() int        { return g.exchangeNeed }
func (g *Game) ExchangeTo() int          { return g.exchangeTo }
func (g *Game) FoulNote(pi int) string   { return g.foulNote[pi] }

// RankPoints は順位ごとの得点 (1 位から順)。Prolog の rank_points/3 由来。
func (g *Game) RankPoints() []int { return g.rankPoints }

// Pile は今の場に出た手を出た順に返す。場が流れると空になる。
func (g *Game) Pile() []PileEntry { return g.pile }

// SetLogger は進行の控えを流す先を決める。nil なら流さない。
// 画面には直近 10 件しか残らないので、あとから読み返したいときに使う。
func (g *Game) SetLogger(logf func(format string, a ...interface{})) { g.logf = logf }

// HandOverNeed は 10捨て・7渡しで選ばせる枚数。
func (g *Game) HandOverNeed() int { return g.handOverNeed }

// HandOverToName は 7渡しの相手の名前。相手がいなければ空。
func (g *Game) HandOverToName() string {
	if g.handOverTo < 0 || g.handOverTo >= len(g.names) {
		return ""
	}
	return g.names[g.handOverTo]
}

// PendingJackBack は今Jバックの可否を決めようとしている手。
func (g *Game) PendingJackBack() []daifugo.Card {
	if g.phase != PhaseJackBack {
		return nil
	}
	return g.sortForDisplay(g.pending.play)
}

// Reversed は今カードの強弱が反転しているか (革命 XOR Jバック)。
func (g *Game) Reversed() bool { return g.revolution != g.jackBack }

// HandCounts は各プレイヤーの残り手札枚数。
func (g *Game) HandCounts() []int {
	out := make([]int, g.players)
	for i := range out {
		out[i] = len(g.hands[i])
	}
	return out
}

// Hand は人間の手札を表示順 (弱い順) で返す。
// 画面の番号はこの並びの 1 始まりの位置。
func (g *Game) Hand() []daifugo.Card { return g.sortForDisplay(g.hands[HumanSeat]) }

// YourTurn は人間のカード選択を待っているか。
func (g *Game) YourTurn() bool {
	return g.phase == PhasePlaying && g.turn == HumanSeat && !g.out[HumanSeat]
}

// CanPass は今パスできるか。場が空なら (親番なので) パスできない。
func (g *Game) CanPass() bool { return g.YourTurn() && !g.field.Empty }

// HasPlayableHand は人間に出せる手があるか。
func (g *Game) HasPlayableHand() (bool, error) {
	if !g.YourTurn() {
		return false, nil
	}
	plays, err := g.engine.LegalPlays(g.hands[HumanSeat], g.field, g.Reversed())
	if err != nil {
		return false, err
	}
	return len(plays) > 0, nil
}

// FieldLabel は場に出ている手の説明 (「♠9 のペア」など)。
func (g *Game) FieldLabel() (string, error) {
	return g.engine.FieldLabel(g.field, g.Reversed())
}

// Winner は累計得点が最大のプレイヤー。
func (g *Game) Winner() int {
	best := 0
	for i, s := range g.scores {
		if s > g.scores[best] {
			best = i
		}
	}
	return best
}

// sortForDisplay は弱い順に並べる。強さは Prolog の card_strength/3 に由来する。
// 同じ強さ (革命で反転しても同順) のときはマークで並びを固定する。
func (g *Game) sortForDisplay(hand []daifugo.Card) []daifugo.Card {
	out := append([]daifugo.Card(nil), hand...)
	strength := map[daifugo.Card]int{}
	for _, c := range out {
		if _, ok := strength[c]; ok {
			continue
		}
		v, err := g.engine.CardStrength(c, g.Reversed())
		if err != nil {
			v = 0
		}
		strength[c] = v
	}
	suitOrder := map[string]int{"s": 0, "h": 1, "d": 2, "c": 3, cards.JokerSuit: 4}
	sort.SliceStable(out, func(i, j int) bool {
		if strength[out[i]] != strength[out[j]] {
			return strength[out[i]] < strength[out[j]]
		}
		return suitOrder[out[i].Suit] < suitOrder[out[j].Suit]
	})
	return out
}

// =============================================
// プレイヤーからの操作
// =============================================

// SubmitExchange は人間が選んだ交換カード (手札の表示番号) を受け取る。
func (g *Game) SubmitExchange(numbers []int) error {
	if g.phase != PhaseExchange {
		return inputErr("今はカードを交換する場面ではありません。")
	}
	if len(numbers) != g.exchangeNeed {
		return inputErr("渡すカードを %d 枚選んでください。", g.exchangeNeed)
	}
	shown := g.Hand()
	seen := map[int]bool{}
	picked := make([]daifugo.Card, 0, len(numbers))
	for _, n := range numbers {
		if n < 1 || n > len(shown) || seen[n] {
			return inputErr("カードの選び方が正しくありません。")
		}
		seen[n] = true
		picked = append(picked, shown[n-1])
	}
	g.pendingGive[HumanSeat] = picked
	return g.applyExchange()
}

// PlayCards は人間が選んだカード (手札の表示番号) を場に出す。
func (g *Game) PlayCards(numbers []int) error {
	if !g.YourTurn() {
		return inputErr("今はあなたの手番ではありません。")
	}
	if len(numbers) == 0 {
		return inputErr("出すカードを選んでください。")
	}
	shown := g.Hand()
	seen := map[int]bool{}
	picked := make([]daifugo.Card, 0, len(numbers))
	for _, n := range numbers {
		if n < 1 || n > len(shown) || seen[n] {
			return inputErr("カードの選び方が正しくありません。")
		}
		seen[n] = true
		picked = append(picked, shown[n-1])
	}

	if g.firstPlayRestricted() {
		ok, err := g.engine.FirstLeadOK(g.hands[HumanSeat], picked)
		if err != nil {
			return err
		}
		if !ok {
			open, err := g.engine.OpeningCard()
			if err != nil {
				return err
			}
			return inputErr("1 ゲーム目は %s の単騎から始めます。", cards.Label(open))
		}
	}

	newField, fx, ok, err := g.engine.ResolvePlay(g.hands[HumanSeat], picked, g.field, g.Reversed())
	if err != nil {
		return err
	}
	if !ok {
		reason, err := g.explainIllegal(picked)
		if err != nil {
			return err
		}
		return ErrPlayerInput{Msg: reason}
	}

	g.notice = ""
	if err := g.commit(HumanSeat, picked, newField, fx); err != nil {
		return err
	}
	return g.step()
}

// Pass は人間がパスする。
func (g *Game) Pass() error {
	if !g.YourTurn() {
		return inputErr("今はあなたの手番ではありません。")
	}
	if g.field.Empty {
		return inputErr("あなたが親なので、必ず何か出してください。")
	}
	g.notice = ""
	g.doPass(HumanSeat)
	return g.step()
}

// Next は 1 ゲームの結果画面から次のゲームへ進める。
func (g *Game) Next() error {
	if g.phase != PhaseRoundEnd {
		return nil
	}
	if g.gameNo >= g.totalGames {
		g.phase = PhaseGameOver
		return nil
	}
	g.gameNo++
	return g.startGame()
}

// =============================================
// 1 ゲームの開始と配札
// =============================================

func (g *Game) startGame() error {
	hands, err := g.deal()
	if err != nil {
		return err
	}
	g.hands = hands
	g.field = daifugo.EmptyField()
	g.lastPlay = nil
	g.pile = nil
	g.revolution = false
	g.jackBack = false
	g.lastPlayer = -1
	g.anyPlayed = false
	g.passedSince = make([]bool, g.players)
	g.locked = make([]bool, g.players)
	g.out = make([]bool, g.players)
	g.finished = nil
	g.penalized = nil
	g.foulNote = map[int]string{}
	g.foulPending = map[int]string{}
	g.pending = pendingPlay{seat: -1}
	g.handOverNeed = 0
	g.handOverTo = -1
	g.discarded = nil
	g.received = nil
	g.events = nil
	g.notice = ""
	g.exchangeNeed = 0
	g.exchangeTo = -1

	if g.gameNo == 1 || len(g.titles) == 0 {
		g.prevDaifugo = -1
		return g.startPlay()
	}
	return g.startExchange()
}

// deal はデッキをシャッフルして配る。
// 配り終えた状態は daifugo.pl の validate_deal/2 で検証する。
func (g *Game) deal() ([][]daifugo.Card, error) {
	deck := append([]daifugo.Card(nil), g.deck...)
	g.rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	sizes, err := g.engine.DealSizes(g.players, len(deck))
	if err != nil {
		return nil, err
	}
	hands := make([][]daifugo.Card, g.players)
	pos := 0
	for i, n := range sizes {
		hands[i] = append([]daifugo.Card(nil), deck[pos:pos+n]...)
		pos += n
	}

	ok, err := g.engine.ValidateDeal(hands, g.deck)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("配札が daifugo.pl の validate_deal/2 を通りませんでした")
	}
	return hands, nil
}

// =============================================
// カード交換
// =============================================

// startExchange は前のゲームの称号に応じたカード交換を始める。
// 下位のプレイヤーは強いカードを必ず渡すので Prolog に選ばせ、
// 上位のプレイヤーは好きに選べるので、人間なら入力を待つ。
func (g *Game) startExchange() error {
	pairs, err := g.engine.Exchanges(g.players)
	if err != nil {
		return err
	}
	g.pendingGive = make([][]daifugo.Card, g.players)
	g.exchangeNeed = 0

	for _, p := range pairs {
		high := g.seatOfRank(p.High)
		low := g.seatOfRank(p.Low)
		if high < 0 || low < 0 {
			continue
		}
		// 下位 → 上位: 必ず強いカードを渡す
		strong, err := g.engine.StrongestCards(g.hands[low], p.Count)
		if err != nil {
			return err
		}
		g.pendingGive[low] = strong

		// 上位 → 下位: 好きなカードを渡す
		if high == HumanSeat {
			g.exchangeNeed = p.Count
			g.exchangeTo = low
			continue
		}
		weak, err := g.engine.WeakestCards(g.hands[high], p.Count)
		if err != nil {
			return err
		}
		g.pendingGive[high] = weak
	}

	if g.exchangeNeed > 0 {
		g.phase = PhaseExchange
		return nil
	}
	return g.applyExchange()
}

// applyExchange は選び終えたカードを一斉に受け渡す。
func (g *Game) applyExchange() error {
	pairs, err := g.engine.Exchanges(g.players)
	if err != nil {
		return err
	}
	g.received = nil

	type move struct{ from, to int }
	var moves []move
	for _, p := range pairs {
		high := g.seatOfRank(p.High)
		low := g.seatOfRank(p.Low)
		if high < 0 || low < 0 {
			continue
		}
		moves = append(moves, move{low, high}, move{high, low})
	}

	// まず全員の手札から抜き、それから配る (同時に交換するため)
	for _, m := range moves {
		g.hands[m.from] = cards.RemoveAll(g.hands[m.from], g.pendingGive[m.from])
	}
	for _, m := range moves {
		g.hands[m.to] = append(g.hands[m.to], g.pendingGive[m.from]...)
		if m.to == HumanSeat {
			g.received = append(g.received, g.pendingGive[m.from]...)
		}
		g.addEvent(m.from, fmt.Sprintf("%s → %s へ %d 枚",
			g.names[m.from], g.names[m.to], len(g.pendingGive[m.from])), nil)
	}
	g.received = g.sortForDisplay(g.received)
	g.pendingGive = nil
	g.exchangeNeed = 0
	return g.startPlay()
}

// seatOfRank は前のゲームで指定の順位だったプレイヤー番号。見つからなければ -1。
func (g *Game) seatOfRank(rank int) int {
	if rank < 1 || rank > len(g.lastRanking) {
		return -1
	}
	return g.lastRanking[rank-1]
}

// =============================================
// 進行
// =============================================

// startPlay は最初の親を決めてカードのやり取りに入る。
func (g *Game) startPlay() error {
	leader, err := g.firstLeader()
	if err != nil {
		return err
	}
	g.turn = leader
	g.lastPlayer = -1
	g.phase = PhasePlaying
	if g.gameNo == 1 {
		g.addEvent(-1, fmt.Sprintf("%s から始めます", g.names[leader]), nil)
	} else {
		g.addEvent(-1, fmt.Sprintf("大貧民の %s から始めます", g.names[leader]), nil)
	}
	return g.step()
}

// firstLeader は最初の親。1 ゲーム目は ♦3 の持ち主 (設定によっては席順の先頭)、
// 2 ゲーム目以降は前のゲームの大貧民。
func (g *Game) firstLeader() (int, error) {
	if g.gameNo > 1 && len(g.lastRanking) == g.players {
		return g.lastRanking[g.players-1], nil
	}
	if g.opts.FirstPlay == "free" {
		return 0, nil
	}
	open, err := g.engine.OpeningCard()
	if err != nil {
		return 0, err
	}
	for i, hand := range g.hands {
		if cards.Contains(hand, open) {
			return i, nil
		}
	}
	return 0, nil
}

// firstPlayRestricted は「♦3 を必ず 1 枚出す」制約が効いている場面か。
// 効くのは 1 ゲーム目のいちばん最初の 1 手だけ。場が流れても復活しない。
func (g *Game) firstPlayRestricted() bool {
	return g.gameNo == 1 && !g.anyPlayed && g.opts.FirstPlay == "diamond3_must"
}

// step は人間の番になるか、ゲームが終わるまで CPU を進める。
func (g *Game) step() error {
	for i := 0; g.phase == PhasePlaying; i++ {
		if i > 1000 {
			return errors.New("進行が終わりませんでした (ルールの組み合わせを確認してください)")
		}
		if g.remaining() <= 1 {
			return g.finishGame()
		}
		if g.turn == HumanSeat && !g.out[HumanSeat] {
			return nil
		}
		if err := g.cpuTurn(g.turn); err != nil {
			return err
		}
	}
	return nil
}

// remaining はまだ手札を持っているプレイヤーの数。
func (g *Game) remaining() int {
	n := 0
	for i := range g.out {
		if !g.out[i] {
			n++
		}
	}
	return n
}

// cpuTurn は CPU 1 人分の手番。合法手からランダムに選ぶ。
func (g *Game) cpuTurn(pi int) error {
	canPass := !g.field.Empty
	seed := g.rng.Intn(1 << 30)

	// 1 ゲーム目の ♦3 縛りだけは、合法手の候補をここで絞る。
	if g.firstPlayRestricted() {
		open, err := g.engine.OpeningCard()
		if err != nil {
			return err
		}
		if cards.Contains(g.hands[pi], open) {
			play := []daifugo.Card{open}
			newField, fx, ok, err := g.engine.ResolvePlay(g.hands[pi], play, g.field, g.Reversed())
			if err != nil {
				return err
			}
			if ok {
				return g.commit(pi, play, newField, fx)
			}
		}
	}

	play, pass, err := g.engine.ChoosePlay(g.hands[pi], g.field, g.Reversed(), canPass, seed)
	if err != nil {
		return err
	}
	if pass || len(play) == 0 {
		if g.field.Empty {
			return fmt.Errorf("%s が親番でパスを選びました", g.names[pi])
		}
		g.doPass(pi)
		return nil
	}
	newField, fx, ok, err := g.engine.ResolvePlay(g.hands[pi], play, g.field, g.Reversed())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s が出そうとした手は daifugo.pl のルールに反します: %v", g.names[pi], play)
	}
	return g.commit(pi, play, newField, fx)
}

// commit は 1 手を場に反映する。人間の入力も CPU の出力も、
// 必ず Prolog の resolve_play/6 を通ってからここへ来る。
//
// 1 手を出したあとの処理は「Jバックの適用 → 10捨て → 7渡し → 上がり判定」
// の順に進む。途中の 3 か所は人間に選ばせる場面があり、そこで一度止まって
// 入力を待つ。止まった位置は pending が覚えていて、Submit* から先へ続く。
func (g *Game) commit(pi int, play []daifugo.Card, newField daifugo.Field, fx daifugo.Effects) error {
	g.hands[pi] = cards.RemoveAll(g.hands[pi], play)
	g.anyPlayed = true
	g.field = newField
	g.lastPlay = g.sortForDisplay(play)
	g.lastPlayer = pi
	for i := range g.passedSince {
		g.passedSince[i] = false
	}
	g.addEvent(pi, g.names[pi]+" が出しました", play)
	g.addPile(pi, play, false)

	if fx.Revolution {
		g.revolution = !g.revolution
		if g.revolution {
			g.addEvent(-1, "革命! 強弱が入れ替わりました", nil)
			g.notice = "革命が起きました。カードの強弱が入れ替わります。"
		} else {
			g.addEvent(-1, "革命返し! 強弱が元に戻りました", nil)
			g.notice = "革命返しです。カードの強弱が元に戻りました。"
		}
	}

	g.pending = pendingPlay{seat: pi, play: play, cut: fx.Cut, jackBack: fx.JackBack}
	return g.jackBackStage()
}

// =============================================
// 出したあとの処理 (Jバック → 10捨て → 7渡し → 上がり)
// =============================================

// jackBackStage は Jバックを適用するかどうかを決める。
// 任意適用の設定なら、出した本人が決める。
func (g *Game) jackBackStage() error {
	p := g.pending
	if !p.jackBack || g.jackBack {
		return g.tenDiscardStage()
	}

	optional, err := g.engine.JackBackOptional()
	if err != nil {
		return err
	}
	if !optional {
		g.applyJackBack()
		return g.tenDiscardStage()
	}
	if p.seat == HumanSeat {
		g.phase = PhaseJackBack
		return nil
	}
	if g.rng.Intn(2) == 0 {
		g.applyJackBack()
	} else {
		g.addEvent(p.seat, g.names[p.seat]+" はJバックを使いませんでした", nil)
	}
	return g.tenDiscardStage()
}

// SubmitJackBack は人間が「Jバックするか」を答える。
func (g *Game) SubmitJackBack(apply bool) error {
	if g.phase != PhaseJackBack {
		return inputErr("今はJバックを決める場面ではありません。")
	}
	g.phase = PhasePlaying
	if apply {
		g.applyJackBack()
	} else {
		g.addEvent(HumanSeat, "あなたはJバックを使いませんでした", nil)
	}
	if err := g.tenDiscardStage(); err != nil {
		return err
	}
	return g.step()
}

func (g *Game) applyJackBack() {
	g.jackBack = true
	g.addEvent(-1, "Jバック! 場が流れるまで強弱が反転します", nil)
	g.notice = "Jバックです。場が流れるまで強弱が反転します。"
}

// tenDiscardStage は 10捨て。出した 10 の枚数だけ手札を捨てる。
func (g *Game) tenDiscardStage() error {
	p := g.pending
	count, err := g.engine.TenDiscardCount(p.play)
	if err != nil {
		return err
	}
	if count == 0 {
		return g.sevenGiveStage()
	}
	if err := g.checkShortage(p.seat, count, "10捨て"); err != nil {
		return err
	}
	count = min(count, len(g.hands[p.seat]))
	if count == 0 {
		return g.sevenGiveStage()
	}

	if p.seat == HumanSeat {
		g.handOverNeed = count
		g.phase = PhaseTenDiscard
		return nil
	}
	picked, err := g.engine.WeakestCards(g.hands[p.seat], count)
	if err != nil {
		return err
	}
	g.applyTenDiscard(p.seat, picked)
	return g.sevenGiveStage()
}

// SubmitTenDiscard は人間が選んだ 10捨てのカード (手札の表示番号)。
func (g *Game) SubmitTenDiscard(numbers []int) error {
	if g.phase != PhaseTenDiscard {
		return inputErr("今はカードを捨てる場面ではありません。")
	}
	picked, err := g.pickFromHand(numbers, g.handOverNeed, "捨てる")
	if err != nil {
		return err
	}
	g.phase = PhasePlaying
	g.handOverNeed = 0
	g.applyTenDiscard(HumanSeat, picked)
	if err := g.sevenGiveStage(); err != nil {
		return err
	}
	return g.step()
}

func (g *Game) applyTenDiscard(pi int, picked []daifugo.Card) {
	g.hands[pi] = cards.RemoveAll(g.hands[pi], picked)
	g.discarded = append(g.discarded, picked...)
	g.addEvent(pi, fmt.Sprintf("%s が10捨てで %d 枚捨てました", g.names[pi], len(picked)), nil)
}

// sevenGiveStage は 7渡し。出した 7 の枚数だけ次の人へ手札を渡す。
func (g *Game) sevenGiveStage() error {
	p := g.pending
	count, err := g.engine.SevenGiveCount(p.play)
	if err != nil {
		return err
	}
	if count == 0 {
		return g.finishPlay()
	}
	if err := g.checkShortage(p.seat, count, "7渡し"); err != nil {
		return err
	}
	count = min(count, len(g.hands[p.seat]))
	to := g.nextAlive((p.seat + 1) % g.players)
	if count == 0 || to == p.seat {
		return g.finishPlay()
	}

	if p.seat == HumanSeat {
		g.handOverNeed = count
		g.handOverTo = to
		g.phase = PhaseSevenGive
		return nil
	}
	picked, err := g.engine.WeakestCards(g.hands[p.seat], count)
	if err != nil {
		return err
	}
	g.applySevenGive(p.seat, to, picked)
	return g.finishPlay()
}

// SubmitSevenGive は人間が選んだ 7渡しのカード (手札の表示番号)。
func (g *Game) SubmitSevenGive(numbers []int) error {
	if g.phase != PhaseSevenGive {
		return inputErr("今はカードを渡す場面ではありません。")
	}
	picked, err := g.pickFromHand(numbers, g.handOverNeed, "渡す")
	if err != nil {
		return err
	}
	to := g.handOverTo
	g.phase = PhasePlaying
	g.handOverNeed = 0
	g.handOverTo = -1
	g.applySevenGive(HumanSeat, to, picked)
	if err := g.finishPlay(); err != nil {
		return err
	}
	return g.step()
}

func (g *Game) applySevenGive(from, to int, picked []daifugo.Card) {
	g.hands[from] = cards.RemoveAll(g.hands[from], picked)
	g.hands[to] = append(g.hands[to], picked...)
	g.addEvent(from, fmt.Sprintf("%s が7渡しで %s へ %d 枚渡しました",
		g.names[from], g.names[to], len(picked)), nil)
	if to == HumanSeat {
		g.received = g.sortForDisplay(picked)
		g.notice = fmt.Sprintf("%s から7渡しで %d 枚受け取りました。", g.names[from], len(picked))
	}
}

// checkShortage は 10捨て・7渡しで手札が足りないかを見る。
// 足りなければ (反則上がりを採用していれば) その場で反則を記録する。
func (g *Game) checkShortage(pi, required int, what string) error {
	foul, err := g.engine.FoulShortage(required, len(g.hands[pi]))
	if err != nil {
		return err
	}
	if !foul {
		return nil
	}
	if _, already := g.foulPending[pi]; already {
		return nil
	}
	g.foulPending[pi] = what + "で手札が足りない"
	g.addEvent(pi, fmt.Sprintf("%s は%sの枚数を出せません (反則)", g.names[pi], what), nil)
	return nil
}

// finishPlay は上がり判定と、次に誰の番になるかの決定。
func (g *Game) finishPlay() error {
	p := g.pending
	g.pending = pendingPlay{seat: -1}

	finished := len(g.hands[p.seat]) == 0
	if finished {
		if err := g.recordFinish(p.seat, p.play); err != nil {
			return err
		}
	}

	switch {
	case p.cut:
		g.addEvent(-1, "場が流れました (8流し・スペ3返し)", nil)
		g.flush(p.seat)
	case finished && g.opts.UsesFinishFlush():
		g.addEvent(-1, "上がったので場を流します", nil)
		g.flush(p.seat)
	default:
		g.advanceTurn()
	}
	return nil
}

// pickFromHand は表示番号のリストを手札のカードに直す。
func (g *Game) pickFromHand(numbers []int, need int, what string) ([]daifugo.Card, error) {
	if len(numbers) != need {
		return nil, inputErr("%sカードを %d 枚選んでください。", what, need)
	}
	shown := g.Hand()
	seen := map[int]bool{}
	picked := make([]daifugo.Card, 0, len(numbers))
	for _, n := range numbers {
		if n < 1 || n > len(shown) || seen[n] {
			return nil, inputErr("カードの選び方が正しくありません。")
		}
		seen[n] = true
		picked = append(picked, shown[n-1])
	}
	return picked, nil
}

// doPass は 1 人分のパス。
func (g *Game) doPass(pi int) {
	g.passedSince[pi] = true
	if g.opts.UsesPassLock() {
		g.locked[pi] = true
	}
	g.addEvent(pi, g.names[pi]+" はパス", nil)
	g.addPile(pi, nil, true)
	g.advanceTurn()
}

// recordFinish は上がりを記録する。反則上がりと都落ちもここで処理する。
func (g *Game) recordFinish(pi int, play []daifugo.Card) error {
	g.out[pi] = true

	foul, err := g.engine.FoulFinish(play, g.Reversed())
	if err != nil {
		return err
	}
	// 10捨て・7渡しの枚数を賄えなかった場合も反則上がり
	note := "反則上がり"
	if pendingNote, ok := g.foulPending[pi]; ok {
		foul = true
		note = pendingNote
	}
	if foul {
		g.penalized = append(g.penalized, pi)
		g.foulNote[pi] = note
		g.addEvent(pi, g.names[pi]+" は反則上がり!", nil)
		g.notice = g.names[pi] + " が反則上がりになりました。"
		return nil
	}

	g.finished = append(g.finished, pi)
	place := len(g.finished)
	g.addEvent(pi, fmt.Sprintf("%s が %d 番目に上がりました", g.names[pi], place), nil)

	// 都落ち: 前のゲームの大富豪が 1 位を守れなかった時点で大貧民になる
	if place == 1 && g.opts.UsesMiyakoOchi() && g.prevDaifugo >= 0 && g.prevDaifugo != pi &&
		!g.out[g.prevDaifugo] {
		fallen := g.prevDaifugo
		g.out[fallen] = true
		g.hands[fallen] = nil
		g.penalized = append(g.penalized, fallen)
		g.foulNote[fallen] = "都落ち"
		g.addEvent(fallen, g.names[fallen]+" は都落ち!", nil)
		g.notice = g.names[fallen] + " は大富豪を守れず都落ちです。"
	}
	return nil
}

// flush は場を流し、leader (またはその次の生存者) を親にする。
func (g *Game) flush(leader int) {
	g.field = daifugo.EmptyField()
	g.lastPlay = nil
	g.pile = nil
	g.jackBack = false
	g.lastPlayer = -1
	for i := range g.passedSince {
		g.passedSince[i] = false
		g.locked[i] = false
	}
	if g.remaining() == 0 {
		return
	}
	g.turn = g.nextAlive(leader)
}

// nextAlive は from 本人が生きていれば from、そうでなければ次の生存者。
func (g *Game) nextAlive(from int) int {
	for step := 0; step < g.players; step++ {
		p := (from + step) % g.players
		if !g.out[p] {
			return p
		}
	}
	return from
}

// advanceTurn は次に手番が回る人を決める。
// 一周して最後に出した人まで戻ってきたら、全員がパスしたということなので場を流す。
func (g *Game) advanceTurn() {
	if g.remaining() <= 1 {
		return
	}
	if g.allPassed() {
		g.flush(g.flushLeader())
		g.addEvent(-1, "全員パスしたので場が流れました", nil)
		return
	}
	for step := 1; step <= g.players; step++ {
		p := (g.turn + step) % g.players
		if g.out[p] {
			continue
		}
		if g.opts.UsesPassLock() && g.locked[p] {
			continue
		}
		if p == g.lastPlayer {
			break
		}
		g.turn = p
		return
	}
	// 出せる人が残っていない
	g.flush(g.flushLeader())
	g.addEvent(-1, "全員パスしたので場が流れました", nil)
}

// allPassed は最後に出した人以外の全員がパス済みか。
func (g *Game) allPassed() bool {
	if g.field.Empty {
		return false
	}
	for p := 0; p < g.players; p++ {
		if g.out[p] || p == g.lastPlayer {
			continue
		}
		if !g.passedSince[p] {
			return false
		}
	}
	return true
}

// flushLeader は場が流れたあとの親。最後に出した人が上がっていれば、その次の人。
func (g *Game) flushLeader() int {
	if g.lastPlayer < 0 {
		return g.turn
	}
	if !g.out[g.lastPlayer] {
		return g.lastPlayer
	}
	return g.nextAlive((g.lastPlayer + 1) % g.players)
}

// =============================================
// ゲーム終了と順位付け
// =============================================

func (g *Game) finishGame() error {
	// まだ手札を持っている人 (最下位) を上がり順の最後に加える
	for p := 0; p < g.players; p++ {
		if !g.out[p] {
			g.out[p] = true
			g.finished = append(g.finished, p)
		}
	}

	// 反則上がり・都落ちは下位から発生順に詰める
	ranking := append([]int(nil), g.finished...)
	for i := len(g.penalized) - 1; i >= 0; i-- {
		ranking = append(ranking, g.penalized[i])
	}

	titles, err := g.engine.Titles(g.players)
	if err != nil {
		return err
	}

	g.lastRanking = ranking
	g.lastTitles = make([]string, g.players)
	g.titles = make([]string, g.players)
	for rank, pi := range ranking {
		if rank < len(titles) {
			g.titles[pi] = titles[rank]
			g.lastTitles[pi] = titles[rank]
		}
		if rank < len(g.rankPoints) {
			g.scores[pi] += g.rankPoints[rank]
		}
	}
	g.prevDaifugo = ranking[0]

	g.phase = PhaseRoundEnd
	return nil
}

// =============================================
// 補助
// =============================================

func (g *Game) addEvent(player int, text string, played []daifugo.Card) {
	e := Event{Player: player, Text: text}
	if len(played) > 0 {
		e.Cards = g.sortForDisplay(played)
	}
	g.events = append(g.events, e)
	if len(g.events) > maxEvents {
		g.events = g.events[len(g.events)-maxEvents:]
	}

	// 画面に残るのは直近 maxEvents 件だけなので、あとから追える控えを
	// 呼び出し側 (サーバーのログ) に流す。
	if g.logf != nil {
		line := text
		if len(e.Cards) > 0 {
			labels := make([]string, len(e.Cards))
			for i, c := range e.Cards {
				labels[i] = cards.Label(c)
			}
			line += " [" + strings.Join(labels, " ") + "]"
		}
		g.logf("第%dゲーム %s", g.gameNo, line)
	}
}

// addPile は今の場に出た手 (またはパス) を記録する。
func (g *Game) addPile(player int, played []daifugo.Card, passed bool) {
	e := PileEntry{Player: player, Passed: passed}
	if len(played) > 0 {
		e.Cards = g.sortForDisplay(played)
	}
	g.pile = append(g.pile, e)
}

// explainIllegal は出せない理由を日本語で説明する。
//
// 判定そのものは Prolog が済ませているので、ここは案内文だけを組み立てる。
// 出せない理由は同時にいくつも重なりうるので、「その手のどこが引っかかって
// いるか」を実際に照らし合わせて、当てはまるものだけを挙げる。
func (g *Game) explainIllegal(play []daifugo.Card) (string, error) {
	// 場が空だったら出せる形か。通れば、その手の種別・枚数・強さ・マークが分かる。
	shape, _, formOK, err := g.engine.ResolvePlay(
		g.hands[HumanSeat], play, daifugo.EmptyField(), g.Reversed())
	if err != nil {
		return "", err
	}
	if !formOK {
		if len(play) >= 5 {
			return "同じ数字を 5 枚以上出すには、ファイブカードの設定が要ります。", nil
		}
		return "その組み合わせは出し方として成立していません" +
			" (同じ数字を揃えるか、同じマークで 3 枚以上連続させてください)。", nil
	}
	if g.field.Empty {
		return "その手は出せません。", nil
	}

	var reasons []string

	if shape.Kind != g.field.Kind {
		reasons = append(reasons, fmt.Sprintf("場は%sなので、%sで出してください",
			kindJa(g.field.Kind), kindJa(g.field.Kind)))
	} else if shape.Len != g.field.Len {
		reasons = append(reasons, fmt.Sprintf("場は %d 枚なので、%d 枚で出してください",
			g.field.Len, g.field.Len))
	}

	if missing := missingSuits(g.field.Bound, shape.Suits); len(missing) > shape.Jokers {
		reasons = append(reasons, fmt.Sprintf("%s で縛られています (%s を含めてください)",
			daifugo.SuitsLabel(g.field.Bound), daifugo.SuitsLabel(missing)))
	}

	if g.field.Strict && shape.Strength != g.field.Strength+1 {
		reasons = append(reasons, "激縛り中なので、ちょうど 1 つ上の数字しか出せません")
	} else if shape.Kind == g.field.Kind && shape.Len == g.field.Len &&
		shape.Strength <= g.field.Strength {
		label, err := g.FieldLabel()
		if err != nil {
			return "", err
		}
		reasons = append(reasons, fmt.Sprintf("場の「%s」より弱い手です", label))
	}

	if len(reasons) == 0 {
		return "そのカードは今のルールでは出せません。", nil
	}

	msg := strings.Join(reasons, "。") + "。"
	if note := g.reversalNote(); note != "" {
		msg += " " + note
	}
	return msg, nil
}

// reversalNote は強弱の向きについての一言。
// 「出せるはずなのに出せない」の多くは、革命や Jバックに気づいていないため。
// 革命と Jバックが重なると反転が打ち消し合うので、そこも書き添える。
func (g *Game) reversalNote() string {
	switch {
	case g.revolution && g.jackBack:
		return "革命中にJバックが重なっているので、強弱は元どおり (2 がいちばん強い) に戻っています。"
	case g.revolution:
		return "革命中なので 3 がいちばん強く、2 がいちばん弱くなっています。"
	case g.jackBack:
		return "Jバック中なので 3 がいちばん強く、2 がいちばん弱くなっています。"
	}
	return ""
}

// ReversalNote は強弱の向きについての一言 (盤面にも出す)。
func (g *Game) ReversalNote() string { return g.reversalNote() }

// PlayableCards は「今の場に出せる手」のどれかに使えるカード。
//
// 合法手の列挙 (daifugo_ai.pl の candidate/2) は、持っている札をあえて
// ジョーカーで置き換えるような手を作らないので、ここに出てこない札が
// 絶対に出せないとまでは言い切れない。あくまで画面の目安として使い、
// 人間が選んだ手は resolve_play/6 が個別に判定する。
func (g *Game) PlayableCards() (map[daifugo.Card]bool, error) {
	if !g.YourTurn() {
		return nil, nil
	}
	plays, err := g.engine.LegalPlays(g.hands[HumanSeat], g.field, g.Reversed())
	if err != nil {
		return nil, err
	}
	out := map[daifugo.Card]bool{}
	for _, p := range plays {
		for _, c := range p {
			out[c] = true
		}
	}
	return out, nil
}

// missingSuits は Bound のうち Suits に無いもの。
func missingSuits(bound, suits []string) []string {
	var out []string
	for _, b := range bound {
		found := false
		for _, s := range suits {
			if s == b {
				found = true
				break
			}
		}
		if !found {
			out = append(out, b)
		}
	}
	return out
}

// kindJa は出し方の種別の日本語表記。
func kindJa(kind string) string {
	if kind == "seq" {
		return "階段"
	}
	return "同じ数字の出し方"
}
