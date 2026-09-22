// Package napoleon はナポレオンのルール判定を提供する。
//
// ハート (internal/rules)・大富豪 (internal/rules/daifugo) と同じ考え方で、
// 判定そのものは全て Prolog 側にある。このパッケージは問い合わせの
// 橋渡しだけを受け持つ。
//
//	napoleon.pl       … 基本ルール (三役を含む強さ、マストフォロー、絵札、ビッド)
//	napoleon_rules.pl … ローカルルール (セイムツー・よろめき・ジョーカー …)
//	napoleon_ai.pl    … CPU の思考
//
// この 3 つの前に internal/plog の compat.pl が読み込まれる。
package napoleon

import (
	"embed"
	"errors"
	"fmt"

	"github.com/ichiban/prolog/engine"

	"hearts/internal/cards"
	"hearts/internal/plog"
)

//go:embed napoleon.pl napoleon_rules.pl napoleon_ai.pl
var prologFS embed.FS

var prologFiles = []string{"napoleon.pl", "napoleon_rules.pl", "napoleon_ai.pl"}

// Card は Prolog の card(Suit, Rank)。ジョーカーは card(joker, jk)。
type Card = cards.Card

// Joker はデッキに入る 1 枚だけのジョーカー。
var Joker = Card{Suit: cards.JokerSuit, Rank: "jk"}

// NoSuit は台札のマークが定まらないとき (ジョーカーがリードで請求なし) の値。
const NoSuit = "none"

// Bid はナポレオン宣言。Count は取ると宣言した絵札の枚数、Suit は切り札。
type Bid struct {
	Count int
	Suit  string
}

// Term は Prolog の項テキストを返す。
func (b Bid) Term() string {
	return fmt.Sprintf("bid(%d,%s)", b.Count, b.Suit)
}

// Label は "♠ で 13 枚" のような表示文字列。
func (b Bid) Label() string {
	return fmt.Sprintf("%s で %d 枚", cards.SuitSymbol(b.Suit), b.Count)
}

// bidTerm は「今の宣言」を表す項。まだ誰も宣言していなければ nil。
func bidTerm(b *Bid) string {
	if b == nil {
		return "nil"
	}
	return b.Term()
}

func leadTerm(lead *Card) string {
	if lead == nil {
		return "nil"
	}
	return lead.Term()
}

// =============================================
// Prolog エンジン
// =============================================

// Engine は Prolog インタプリタを包み、ナポレオンに必要な問い合わせを提供する。
// 並行安全ではない。対局ごとに 1 つ作ること。
type Engine struct {
	p *plog.Interp
}

// New は Prolog を起動し、ルールファイル群を読み込む。
func New(dir string) (*Engine, error) {
	p, err := plog.New(prologFS, dir, prologFiles)
	if err != nil {
		return nil, err
	}
	return &Engine{p: p}, nil
}

// --- 設定 ---

// Players は今の人数。
func (e *Engine) Players() (int, error) {
	var v struct{ N int }
	err := e.p.Scan("players(N).", &v)
	return v.N, err
}

// HandSize は 1 人あたりの手札枚数 (= トリックの数)。
func (e *Engine) HandSize() (int, error) {
	var v struct{ S int }
	err := e.p.Scan("hand_size(S).", &v)
	return v.S, err
}

// WidowSize は場に伏せる埋め札の枚数。
func (e *Engine) WidowSize() (int, error) {
	var v struct{ S int }
	err := e.p.Scan("widow_size(S).", &v)
	return v.S, err
}

// MinBid は最低宣言枚数。
func (e *Engine) MinBid() (int, error) {
	var v struct{ N int }
	err := e.p.Scan("min_bid(N).", &v)
	return v.N, err
}

// UsesJoker はジョーカーを入れる設定か。
func (e *Engine) UsesJoker() (bool, error) {
	return e.p.Prove("uses_joker.")
}

// TotalPointCards は 1 ゲームに存在する絵札の数 (20)。
func (e *Engine) TotalPointCards() (int, error) {
	var v struct{ N int }
	err := e.p.Scan("total_point_cards(N).", &v)
	return v.N, err
}

// --- デッキと配札 ---

// Deck は napoleon_rules.pl の deck/1。
func (e *Engine) Deck() ([]Card, error) {
	var v struct{ Deck []Card }
	err := e.p.Scan("deck(Deck).", &v)
	return v.Deck, err
}

// ValidateDeal は配られた状態が正当かを napoleon.pl の validate_deal/3 で確かめる。
func (e *Engine) ValidateDeal(hands [][]Card, widow, deck []Card) (bool, error) {
	return e.p.Prove(fmt.Sprintf("validate_deal(%s, %s, %s).",
		cards.ListOfLists(hands), cards.List(widow), cards.List(deck)))
}

// --- ビッド ---

// BidOK は宣言が今の宣言を上回っているか。
func (e *Engine) BidOK(bid Bid, current *Bid) (bool, error) {
	return e.p.Prove(fmt.Sprintf("bid_ok(%d, %s, %s).", bid.Count, bid.Suit, bidTerm(current)))
}

// HigherBids は今の宣言を上回る宣言を弱い順に返す。
func (e *Engine) HigherBids(current *Bid) ([]Bid, error) {
	var v struct{ Bids []bidTermScan }
	err := e.p.Scan(fmt.Sprintf("higher_bids(%s, Bids).", bidTerm(current)), &v)
	if err != nil {
		return nil, err
	}
	out := make([]Bid, len(v.Bids))
	for i, b := range v.Bids {
		out[i] = Bid(b)
	}
	return out, nil
}

// ChooseBid は CPU の宣言。pass が真ならパス。
func (e *Engine) ChooseBid(hand []Card, current *Bid, seed int) (Bid, bool, error) {
	q := fmt.Sprintf("choose_bid(%s, %s, %d, Choice).", cards.List(hand), bidTerm(current), seed)
	var v struct{ Choice bidChoice }
	err := e.p.Scan(q, &v)
	if errors.Is(err, plog.ErrNoSolution) {
		return Bid{}, true, nil
	}
	if err != nil {
		return Bid{}, false, err
	}
	return v.Choice.bid, v.Choice.pass, nil
}

var (
	atomBid  = engine.NewAtom("bid")
	atomPass = engine.NewAtom("pass")
)

// bidTermScan は bid(枚数, マーク) を読む。
type bidTermScan Bid

func (b *bidTermScan) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	cmp, ok := env.Resolve(term).(engine.Compound)
	if !ok || cmp.Functor() != atomBid || cmp.Arity() != 2 {
		return fmt.Errorf("bid/2 ではない項です: %v", env.Resolve(term))
	}
	count, ok := cards.AtomOrIntText(cmp.Arg(0), env)
	if !ok {
		return fmt.Errorf("宣言枚数を解釈できません")
	}
	suit, ok := cards.AtomOrIntText(cmp.Arg(1), env)
	if !ok {
		return fmt.Errorf("切り札のマークを解釈できません")
	}
	n := 0
	if _, err := fmt.Sscanf(count, "%d", &n); err != nil {
		return err
	}
	b.Count, b.Suit = n, suit
	return nil
}

// bidChoice は pass / bid(枚数, マーク) のどちらかを読む。
type bidChoice struct {
	pass bool
	bid  Bid
}

func (c *bidChoice) Scan(vm *engine.VM, term engine.Term, env *engine.Env) error {
	if a, ok := env.Resolve(term).(engine.Atom); ok && a == atomPass {
		c.pass = true
		return nil
	}
	var b bidTermScan
	if err := b.Scan(vm, term, env); err != nil {
		return err
	}
	c.pass, c.bid = false, Bid(b)
	return nil
}

// --- 副官の指名と埋め札 ---

// AideCandidates はナポレオンが副官として指名できるカードの候補。
func (e *Engine) AideCandidates(hand []Card, trump string) ([]Card, error) {
	var v struct{ Cards []Card }
	err := e.p.Scan(fmt.Sprintf("aide_candidates(%s, %s, Cards).", cards.List(hand), trump), &v)
	return v.Cards, err
}

// ChooseAide は CPU のナポレオンが指名する副官カード。
func (e *Engine) ChooseAide(hand []Card, trump string, seed int) (Card, error) {
	var v struct{ C Card }
	err := e.p.Scan(fmt.Sprintf("choose_aide(%s, %s, %d, C).", cards.List(hand), trump, seed), &v)
	return v.C, err
}

// ChooseDiscard は CPU のナポレオンが埋め札として伏せ直すカード。
func (e *Engine) ChooseDiscard(hand []Card, count int, trump string) ([]Card, error) {
	var v struct{ Cards []Card }
	err := e.p.Scan(fmt.Sprintf("choose_discard(%s, %d, %s, Cards).",
		cards.List(hand), count, trump), &v)
	return v.Cards, err
}

// --- 手 ---

// LeadSuit は台札のマークを返す。ジョーカーがリードのときは設定で変わる。
func (e *Engine) LeadSuit(lead *Card, trump string) (string, error) {
	var v struct{ S string }
	err := e.p.Scan(fmt.Sprintf("lead_suit_of(%s, %s, S).", leadTerm(lead), trump), &v)
	return v.S, err
}

// LegalCards は今出せるカードの一覧。
func (e *Engine) LegalCards(hand []Card, lead *Card, trump string) ([]Card, error) {
	var v struct{ Cards []Card }
	err := e.p.Scan(fmt.Sprintf("legal_cards(%s, %s, %s, Cards).",
		cards.List(hand), leadTerm(lead), trump), &v)
	return v.Cards, err
}

// IsLegal は 1 枚のカードが出せるか。
func (e *Engine) IsLegal(hand []Card, lead *Card, trump string, card Card) (bool, error) {
	return e.p.Prove(fmt.Sprintf("legal_play(%s, %s, %s, %s).",
		cards.List(hand), leadTerm(lead), trump, card.Term()))
}

// ChooseCard は CPU の選択。合法手から seed に基づいて 1 枚選ぶ。
// trick は今このトリックに出ているカード (出た順)。セイムツーを狙えるかの
// 判断に使う。
func (e *Engine) ChooseCard(hand, trick []Card, lead *Card, trump string, seed int) (Card, error) {
	var v struct{ C Card }
	err := e.p.Scan(fmt.Sprintf("choose_card(%s, %s, %s, %s, %d, C).",
		cards.List(hand), cards.List(trick), leadTerm(lead), trump, seed), &v)
	return v.C, err
}

// --- トリック ---

// TrickWinner はトリックの勝ち札と、その理由 (same_two / yoromeki /
// yoromeki_return / normal) を返す。
func (e *Engine) TrickWinner(trick []Card, trump, leadSuit string) (Card, string, error) {
	var v struct {
		W      Card
		Reason string
	}
	err := e.p.Scan(fmt.Sprintf("trick_winner(%s, %s, %s, W, Reason).",
		cards.List(trick), trump, leadSuit), &v)
	return v.W, v.Reason, err
}

// CountPoints はカード束に含まれる絵札の数。
func (e *Engine) CountPoints(cs []Card) (int, error) {
	var v struct{ N int }
	err := e.p.Scan(fmt.Sprintf("count_points(%s, N).", cards.List(cs)), &v)
	return v.N, err
}

// --- 精算 ---

// Result は 1 ゲームの精算結果。
type Result struct {
	Outcome  string // "napoleon" (ナポレオン軍の勝ち) または "allied"
	Napoleon int
	Aide     int
	Allied   int // 連合軍 1 人あたり
}

// RoundScores は宣言枚数と獲得した絵札の数から得点を求める。
// solo が真なら副官なし (ナポレオン 1 人 vs 連合軍)。
func (e *Engine) RoundScores(bid, taken int, solo bool) (Result, error) {
	q := fmt.Sprintf("round_scores(%d, %d, %s, Outcome, Nap, Aide, Allied).",
		bid, taken, plog.Atom(solo))
	var v struct {
		Outcome string
		Nap     int
		Aide    int
		Allied  int
	}
	if err := e.p.Scan(q, &v); err != nil {
		return Result{}, err
	}
	return Result{Outcome: v.Outcome, Napoleon: v.Nap, Aide: v.Aide, Allied: v.Allied}, nil
}

// CounterJack は裏ジャック (切り札と同じ色のもう一方の J)。
func (e *Engine) CounterJack(trump string) (Card, error) {
	var v struct{ C Card }
	err := e.p.Scan(fmt.Sprintf("counter_jack(%s, C).", trump), &v)
	return v.C, err
}

// MainJack は正ジャック (切り札と同じマークの J)。
func (e *Engine) MainJack(trump string) (Card, error) {
	var v struct{ C Card }
	err := e.p.Scan(fmt.Sprintf("main_jack(%s, C).", trump), &v)
	return v.C, err
}
