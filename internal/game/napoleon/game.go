// Package napoleon はナポレオン 1 卓分の進行を受け持つ。
//
// internal/game (ハート)・internal/game/daifugo と同じ考え方で、画面や HTTP の
// ことは一切知らず、ルール判定も自分では書かない。「出せるか」「どの宣言が
// 通るか」「トリックの勝者は誰か」「何点か」は全て rules/napoleon.Engine
// 経由で Prolog に尋ねる。
package napoleon

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"hearts/internal/cards"
	"hearts/internal/playerr"
	"hearts/internal/rules/napoleon"
)

// Phase は今プレイヤーに何を求めている局面か。
type Phase string

const (
	PhaseBidding    Phase = "bidding"    // ナポレオン宣言 (競り)
	PhaseNaming     Phase = "naming"     // ナポレオンが副官のカードを指名する
	PhaseDiscarding Phase = "discarding" // ナポレオンが埋め札と入れ替える
	PhasePlaying    Phase = "playing"    // 出すカードの選択待ち
	PhaseTrickDone  Phase = "trick_done" // トリック終了、次へ進む待ち
	PhaseRoundDone  Phase = "round_done" // 1 ゲーム終了、精算の確認待ち
	PhaseGameOver   Phase = "game_over"  // 全ゲーム終了
)

// HumanSeat は人間のプレイヤー番号。残りは CPU。
const HumanSeat = 0

// maxRedeals は「全員パス」で配り直す回数の上限。
// これを超えたら、最初のプレイヤーが最低枚数で宣言したものとして進める。
const maxRedeals = 20

// ErrPlayerInput は「入力がルール上おかしい」ことを表す。
// 3 つのゲームで同じ型を使う (internal/playerr)。
type ErrPlayerInput = playerr.Error

func inputErr(format string, a ...interface{}) error {
	return playerr.Errorf(format, a...)
}

// 勝ち方の理由の日本語表記。Prolog の trick_winner/5 が返すアトムに対応する。
var reasonJa = map[string]string{
	"same_two":        "セイムツー",
	"yoromeki":        "よろめき",
	"yoromeki_return": "よろめき返し",
}

// ReasonJa は特殊な勝ち方の日本語表記。通常の勝ちなら空文字。
func ReasonJa(reason string) string { return reasonJa[reason] }

// Game は複数ゲーム通しの 1 卓分の状態。並行安全ではない。
type Game struct {
	engine *napoleon.Engine
	rng    *rand.Rand
	opts   napoleon.Options

	players    int
	handSize   int
	widowSize  int
	maxPoints  int // 絵札の総数 (20)
	totalGames int

	names  []string
	scores []int

	gameNo int
	phase  Phase

	hands [][]napoleon.Card
	widow []napoleon.Card // 場に伏せたカード
	deck  []napoleon.Card

	dealer int

	// 競り
	bid        *napoleon.Bid
	bidder     int // 今の宣言をした人。-1 なら誰もいない
	bidPassed  []bool
	bidTurn    int
	bidHistory []string
	redeals    int

	// 役割
	napoleonSeat int
	aideSeat     int // 未判明・不在なら -1
	aideCard     napoleon.Card
	aideRevealed bool
	solo         bool
	trump        string

	// トリック
	trickNo  int
	leader   int
	turn     int
	played   []napoleon.Card
	playedBy []int
	lead     *napoleon.Card
	leadSuit string
	taken    [][]napoleon.Card

	trickWinner int
	trickReason string
	trickPoints int

	// 精算
	result      napoleon.Result
	roundTaken  int
	roundDeltas []int

	notice string
}

// New は選んだローカルルールを Prolog に反映してゲームを始める。
func New(engine *napoleon.Engine, rng *rand.Rand, totalGames int, opts napoleon.Options) (*Game, error) {
	if err := opts.Apply(engine); err != nil {
		return nil, err
	}
	players, err := engine.Players()
	if err != nil {
		return nil, err
	}
	handSize, err := engine.HandSize()
	if err != nil {
		return nil, err
	}
	widowSize, err := engine.WidowSize()
	if err != nil {
		return nil, err
	}
	maxPoints, err := engine.TotalPointCards()
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
		engine:       engine,
		rng:          rng,
		opts:         opts,
		players:      players,
		handSize:     handSize,
		widowSize:    widowSize,
		maxPoints:    maxPoints,
		totalGames:   totalGames,
		names:        names,
		scores:       make([]int, players),
		deck:         deck,
		gameNo:       1,
		dealer:       players - 1,
		napoleonSeat: -1,
		aideSeat:     -1,
		bidder:       -1,
	}
	if err := g.startRound(); err != nil {
		return nil, err
	}
	return g, nil
}

// =============================================
// 参照 (画面を組み立てる側から読む)
// =============================================

func (g *Game) Phase() Phase              { return g.phase }
func (g *Game) GameNo() int               { return g.gameNo }
func (g *Game) TotalGames() int           { return g.totalGames }
func (g *Game) Players() int              { return g.players }
func (g *Game) Names() []string           { return g.names }
func (g *Game) Scores() []int             { return g.scores }
func (g *Game) Options() napoleon.Options { return g.opts }
func (g *Game) Trump() string             { return g.trump }
func (g *Game) Bid() *napoleon.Bid        { return g.bid }
func (g *Game) BidHistory() []string      { return g.bidHistory }
func (g *Game) BidTurn() int              { return g.bidTurn }
func (g *Game) NapoleonSeat() int         { return g.napoleonSeat }
func (g *Game) AideSeat() int             { return g.aideSeat }
func (g *Game) AideCard() napoleon.Card   { return g.aideCard }
func (g *Game) AideRevealed() bool        { return g.aideRevealed }
func (g *Game) Solo() bool                { return g.solo }
func (g *Game) TrickNo() int              { return g.trickNo }
func (g *Game) TrickTotal() int           { return g.handSize }
func (g *Game) Played() []napoleon.Card   { return g.played }
func (g *Game) PlayedBy() []int           { return g.playedBy }
func (g *Game) Lead() *napoleon.Card      { return g.lead }
func (g *Game) LeadSuit() string          { return g.leadSuit }
func (g *Game) TrickWinner() int          { return g.trickWinner }
func (g *Game) TrickReason() string       { return g.trickReason }
func (g *Game) TrickPoints() int          { return g.trickPoints }
func (g *Game) Turn() int                 { return g.turn }
func (g *Game) Notice() string            { return g.notice }
func (g *Game) MaxPoints() int            { return g.maxPoints }
func (g *Game) Result() napoleon.Result   { return g.result }
func (g *Game) RoundTaken() int           { return g.roundTaken }
func (g *Game) RoundDeltas() []int        { return g.roundDeltas }
func (g *Game) WidowSize() int            { return g.widowSize }

// Hand は人間の手札を表示順 (♠♥♦♣ の順、各マーク内は強い順) で返す。
func (g *Game) Hand() []napoleon.Card { return g.sortForDisplay(g.hands[HumanSeat]) }

// YourTurn は人間のカード選択を待っているか。
func (g *Game) YourTurn() bool { return g.phase == PhasePlaying && g.turn == HumanSeat }

// LegalCards は今この瞬間に人間が出せるカード。
func (g *Game) LegalCards() ([]napoleon.Card, error) {
	if !g.YourTurn() {
		return nil, nil
	}
	return g.engine.LegalCards(g.hands[HumanSeat], g.lead, g.trump)
}

// TakenPoints は各プレイヤーがこのゲームで取った絵札の数。
func (g *Game) TakenPoints() ([]int, error) {
	out := make([]int, g.players)
	for i := range out {
		n, err := g.engine.CountPoints(g.taken[i])
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// NapoleonPoints はナポレオン軍がこれまでに取った絵札の数。
func (g *Game) NapoleonPoints() (int, error) {
	pts, err := g.TakenPoints()
	if err != nil {
		return 0, err
	}
	total := 0
	if g.napoleonSeat >= 0 {
		total += pts[g.napoleonSeat]
	}
	if g.aideSeat >= 0 && g.aideSeat != g.napoleonSeat {
		total += pts[g.aideSeat]
	}
	return total, nil
}

// AideChoices はナポレオンが副官として指名できるカード (手札以外の全て)。
func (g *Game) AideChoices() []napoleon.Card {
	out := make([]napoleon.Card, 0, len(g.deck))
	for _, c := range g.deck {
		if !cards.Contains(g.hands[g.napoleonSeat], c) {
			out = append(out, c)
		}
	}
	return g.sortForDisplay(out)
}

// AvailableBids は人間が今できる宣言。
func (g *Game) AvailableBids() ([]napoleon.Bid, error) {
	return g.engine.HigherBids(g.bid)
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

// sortForDisplay は ♠♥♦♣ の順、各マーク内はランクの弱い順に並べる。
// 強いカード (A) が右端に来る並びで、ジョーカーは末尾。
func (g *Game) sortForDisplay(hand []napoleon.Card) []napoleon.Card {
	out := append([]napoleon.Card(nil), hand...)
	suitOrder := map[string]int{"s": 0, "h": 1, "d": 2, "c": 3, cards.JokerSuit: 4}
	rankOrder := map[string]int{
		"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9,
		"10": 10, "j": 11, "q": 12, "k": 13, "a": 14,
	}
	sort.SliceStable(out, func(i, j int) bool {
		if suitOrder[out[i].Suit] != suitOrder[out[j].Suit] {
			return suitOrder[out[i].Suit] < suitOrder[out[j].Suit]
		}
		return rankOrder[out[i].Rank] < rankOrder[out[j].Rank]
	})
	return out
}

// =============================================
// プレイヤーからの操作
// =============================================

// SubmitBid は人間の宣言。pass が真ならパス。
func (g *Game) SubmitBid(count int, suit string, pass bool) error {
	if g.phase != PhaseBidding || g.bidTurn != HumanSeat {
		return inputErr("今はあなたが宣言する場面ではありません。")
	}
	if pass {
		return g.applyBid(HumanSeat, napoleon.Bid{}, true)
	}
	bid := napoleon.Bid{Count: count, Suit: suit}
	ok, err := g.engine.BidOK(bid, g.bid)
	if err != nil {
		return err
	}
	if !ok {
		if g.bid == nil {
			min, err := g.engine.MinBid()
			if err != nil {
				return err
			}
			return inputErr("宣言は %d 枚以上にしてください。", min)
		}
		return inputErr("今の宣言「%s」より強い宣言が必要です。", g.bid.Label())
	}
	return g.applyBid(HumanSeat, bid, false)
}

// SubmitAide は人間のナポレオンが指名する副官のカード。
func (g *Game) SubmitAide(suit, rank string) error {
	if g.phase != PhaseNaming || g.napoleonSeat != HumanSeat {
		return inputErr("今は副官を指名する場面ではありません。")
	}
	card := napoleon.Card{Suit: suit, Rank: rank}
	if !cards.Contains(g.deck, card) {
		return inputErr("そのカードは使っていません。")
	}
	g.aideCard = card
	return g.startDiscard()
}

// SubmitDiscard は人間のナポレオンが埋め札として伏せ直すカード (表示番号)。
func (g *Game) SubmitDiscard(numbers []int) error {
	if g.phase != PhaseDiscarding || g.napoleonSeat != HumanSeat {
		return inputErr("今は埋め札を選ぶ場面ではありません。")
	}
	if len(numbers) != g.widowSize {
		return inputErr("伏せるカードを %d 枚選んでください。", g.widowSize)
	}
	shown := g.Hand()
	seen := map[int]bool{}
	picked := make([]napoleon.Card, 0, len(numbers))
	for _, n := range numbers {
		if n < 1 || n > len(shown) || seen[n] {
			return inputErr("カードの選び方が正しくありません。")
		}
		seen[n] = true
		picked = append(picked, shown[n-1])
	}
	g.hands[HumanSeat] = cards.RemoveAll(g.hands[HumanSeat], picked)
	g.widow = picked
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

	ok, err := g.engine.IsLegal(g.hands[HumanSeat], g.lead, g.trump, card)
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

// Next はトリック終了・ゲーム終了の画面から先へ進める。
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
			g.leadSuit = ""
			g.notice = ""
			g.phase = PhasePlaying
			return g.advance()
		}
		return g.finishRound()

	case PhaseRoundDone:
		if g.gameNo >= g.totalGames {
			g.phase = PhaseGameOver
			return nil
		}
		g.gameNo++
		return g.startRound()
	}
	return nil
}

// =============================================
// 1 ゲームの開始
// =============================================

func (g *Game) startRound() error {
	g.dealer = (g.dealer + 1) % g.players
	g.redeals = 0
	return g.deal()
}

func (g *Game) deal() error {
	deck := append([]napoleon.Card(nil), g.deck...)
	g.rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	hands := make([][]napoleon.Card, g.players)
	for i := 0; i < g.players; i++ {
		hands[i] = append([]napoleon.Card(nil), deck[i*g.handSize:(i+1)*g.handSize]...)
	}
	widow := append([]napoleon.Card(nil), deck[g.players*g.handSize:]...)

	ok, err := g.engine.ValidateDeal(hands, widow, g.deck)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("配札が napoleon.pl の validate_deal/3 を通りませんでした")
	}

	g.hands = hands
	g.widow = widow
	g.taken = make([][]napoleon.Card, g.players)
	g.napoleonSeat = -1
	g.aideSeat = -1
	g.aideCard = napoleon.Card{}
	g.aideRevealed = false
	g.solo = false
	g.trump = ""
	g.trickNo = 1
	g.played = nil
	g.playedBy = nil
	g.lead = nil
	g.leadSuit = ""
	g.roundDeltas = nil
	g.notice = ""

	return g.startBidding()
}

// =============================================
// 競り (ナポレオン宣言)
// =============================================

func (g *Game) startBidding() error {
	g.bid = nil
	g.bidder = -1
	g.bidPassed = make([]bool, g.players)
	g.bidTurn = (g.dealer + 1) % g.players
	g.bidHistory = nil
	g.phase = PhaseBidding
	return g.advanceBidding()
}

// advanceBidding は人間の番になるまで CPU の宣言を進める。
func (g *Game) advanceBidding() error {
	for g.phase == PhaseBidding {
		if g.bidTurn == HumanSeat {
			return nil
		}
		seed := g.rng.Intn(1 << 30)
		bid, pass, err := g.engine.ChooseBid(g.hands[g.bidTurn], g.bid, seed)
		if err != nil {
			return err
		}
		if err := g.applyBid(g.bidTurn, bid, pass); err != nil {
			return err
		}
	}
	return nil
}

// applyBid は 1 人分の宣言を反映し、競りが決着したかを見る。
func (g *Game) applyBid(pi int, bid napoleon.Bid, pass bool) error {
	if pass {
		g.bidPassed[pi] = true
		g.bidHistory = append(g.bidHistory, g.names[pi]+": パス")
	} else {
		g.bid = &napoleon.Bid{Count: bid.Count, Suit: bid.Suit}
		g.bidder = pi
		g.bidHistory = append(g.bidHistory, g.names[pi]+": "+bid.Label())
	}

	active := 0
	for i := range g.bidPassed {
		if !g.bidPassed[i] {
			active++
		}
	}

	switch {
	case g.bid != nil && active <= 1:
		return g.startNaming()
	case g.bid == nil && active == 0:
		return g.redeal()
	}

	for step := 1; step <= g.players; step++ {
		p := (pi + step) % g.players
		if !g.bidPassed[p] {
			g.bidTurn = p
			break
		}
	}
	return g.advanceBidding()
}

// redeal は誰も宣言しなかったときの配り直し。
func (g *Game) redeal() error {
	g.redeals++
	if g.redeals > maxRedeals {
		// いつまでも決まらない場合の保険。最低枚数で ♣ を宣言したことにする。
		min, err := g.engine.MinBid()
		if err != nil {
			return err
		}
		g.bid = &napoleon.Bid{Count: min, Suit: "c"}
		g.bidder = (g.dealer + 1) % g.players
		g.bidHistory = append(g.bidHistory,
			fmt.Sprintf("誰も宣言しないので %s が %s で引き受けます",
				g.names[g.bidder], g.bid.Label()))
		return g.startNaming()
	}
	g.notice = "誰もナポレオン宣言をしなかったので配り直します。"
	return g.deal()
}

// =============================================
// 副官の指名と埋め札の入れ替え
// =============================================

func (g *Game) startNaming() error {
	g.napoleonSeat = g.bidder
	g.trump = g.bid.Suit
	g.notice = fmt.Sprintf("%s がナポレオンです (%s)。", g.names[g.napoleonSeat], g.bid.Label())

	if g.napoleonSeat == HumanSeat {
		g.phase = PhaseNaming
		return nil
	}
	seed := g.rng.Intn(1 << 30)
	card, err := g.engine.ChooseAide(g.hands[g.napoleonSeat], g.trump, seed)
	if err != nil {
		return err
	}
	g.aideCard = card
	return g.startDiscard()
}

// startDiscard はナポレオンに埋め札を渡し、不要な札を伏せ直させる。
func (g *Game) startDiscard() error {
	g.hands[g.napoleonSeat] = append(g.hands[g.napoleonSeat], g.widow...)
	g.widow = nil

	if g.napoleonSeat == HumanSeat {
		g.phase = PhaseDiscarding
		return nil
	}
	discard, err := g.engine.ChooseDiscard(g.hands[g.napoleonSeat], g.widowSize, g.trump)
	if err != nil {
		return err
	}
	g.hands[g.napoleonSeat] = cards.RemoveAll(g.hands[g.napoleonSeat], discard)
	g.widow = discard
	return g.startTricks()
}

// =============================================
// トリックプレイ
// =============================================

// startTricks は副官を確定させてからトリックプレイに入る。
// 指名カードがナポレオンの手札にある (または埋め札に伏せられた) 場合は副官なし。
func (g *Game) startTricks() error {
	g.aideSeat = -1
	for i, hand := range g.hands {
		if i == g.napoleonSeat {
			continue
		}
		if cards.Contains(hand, g.aideCard) {
			g.aideSeat = i
			break
		}
	}
	g.solo = g.aideSeat < 0
	if g.solo {
		g.aideRevealed = true
		g.notice = fmt.Sprintf("%s の指名は %s。持ち主がいないので副官なしのゲームです。",
			g.names[g.napoleonSeat], cards.Label(g.aideCard))
	} else {
		g.notice = fmt.Sprintf("%s は副官として %s を指名しました。",
			g.names[g.napoleonSeat], cards.Label(g.aideCard))
	}

	g.leader = g.napoleonSeat
	g.turn = g.leader
	g.trickNo = 1
	g.played = nil
	g.playedBy = nil
	g.lead = nil
	g.leadSuit = ""
	g.phase = PhasePlaying
	return g.advance()
}

// advance は人間の手番になるかトリックが揃うまで CPU を進める。
func (g *Game) advance() error {
	if g.phase != PhasePlaying {
		return nil
	}
	for len(g.played) < g.players && g.turn != HumanSeat {
		seed := g.rng.Intn(1 << 30)
		card, err := g.engine.ChooseCard(g.hands[g.turn], g.played, g.lead, g.trump, seed)
		if err != nil {
			return err
		}
		if err := g.commit(g.turn, card); err != nil {
			return err
		}
	}
	if len(g.played) == g.players {
		return g.resolveTrick()
	}
	return nil
}

// commit は 1 枚を場に出す。人間の入力も CPU の出力も、最後に必ず
// Prolog の legal_play/4 を通るかどうかを確かめてから反映する。
func (g *Game) commit(pi int, card napoleon.Card) error {
	ok, err := g.engine.IsLegal(g.hands[pi], g.lead, g.trump, card)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s が出そうとした %s は napoleon.pl のルールに反します",
			g.names[pi], cards.Label(card))
	}

	g.hands[pi] = cards.Remove(g.hands[pi], card)
	g.played = append(g.played, card)
	g.playedBy = append(g.playedBy, pi)
	if len(g.played) == 1 {
		c := card
		g.lead = &c
		suit, err := g.engine.LeadSuit(&c, g.trump)
		if err != nil {
			return err
		}
		g.leadSuit = suit
	}

	// 副官のカードが出たら正体が分かる
	if !g.aideRevealed && !g.solo && card == g.aideCard {
		g.aideRevealed = true
		g.notice = fmt.Sprintf("%s が %s を出しました。%s が副官です！",
			g.names[pi], cards.Label(card), g.names[pi])
	}

	g.turn = (pi + 1) % g.players
	return nil
}

func (g *Game) resolveTrick() error {
	win, reason, err := g.engine.TrickWinner(g.played, g.trump, g.leadSuit)
	if err != nil {
		return err
	}
	i := cards.IndexOf(g.played, win)
	if i < 0 {
		return fmt.Errorf("勝ち札 %s が場にありません", cards.Label(win))
	}
	winner := g.playedBy[i]
	g.taken[winner] = append(g.taken[winner], g.played...)
	g.trickWinner = winner
	g.trickReason = reason

	g.trickPoints, err = g.engine.CountPoints(g.played)
	if err != nil {
		return err
	}
	if label := ReasonJa(reason); label != "" {
		g.notice = fmt.Sprintf("%s！ %s が %s で取りました。",
			label, g.names[winner], cards.Label(win))
	}

	g.phase = PhaseTrickDone
	return nil
}

// =============================================
// 精算
// =============================================

func (g *Game) finishRound() error {
	taken, err := g.NapoleonPoints()
	if err != nil {
		return err
	}
	g.roundTaken = taken

	result, err := g.engine.RoundScores(g.bid.Count, taken, g.solo)
	if err != nil {
		return err
	}
	g.result = result

	g.roundDeltas = make([]int, g.players)
	for i := 0; i < g.players; i++ {
		switch {
		case i == g.napoleonSeat:
			g.roundDeltas[i] = result.Napoleon
		case !g.solo && i == g.aideSeat:
			g.roundDeltas[i] = result.Aide
		default:
			g.roundDeltas[i] = result.Allied
		}
		g.scores[i] += g.roundDeltas[i]
	}

	// 副官は精算で必ず明かす
	g.aideRevealed = true
	g.phase = PhaseRoundDone
	return nil
}

// =============================================
// 補助
// =============================================

// explainIllegal は出せない理由を日本語で説明する。
// 判定そのものは Prolog が済ませているので、ここは案内文だけを組み立てる。
func (g *Game) explainIllegal(card napoleon.Card) (string, error) {
	if g.lead == nil {
		return "そのカードはルール上出せません。", nil
	}
	if g.opts.JokerDemand == "on" && *g.lead == (napoleon.Card{Suit: "s", Rank: "3"}) {
		for _, c := range g.hands[HumanSeat] {
			if c.IsJoker() {
				return "♠3 が台札なのでジョーカーを出してください (ジョーカー請求)。", nil
			}
		}
	}
	if g.lead.IsJoker() && g.opts.JokerLead == "trump" {
		return fmt.Sprintf("ジョーカーが台札なので、%sの切り札を出してください (切り札請求)。",
			cards.SuitName(g.trump)), nil
	}
	if g.leadSuit != napoleon.NoSuit && card.Suit != g.leadSuit {
		return fmt.Sprintf("台札は%sです。%sを持っているので必ず出してください。",
			cards.SuitName(g.leadSuit), cards.SuitName(g.leadSuit)), nil
	}
	return "そのカードはルール上出せません。", nil
}
