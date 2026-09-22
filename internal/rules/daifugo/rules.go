// Package daifugo は大富豪のルール判定を提供する。
//
// ハート (internal/rules) と同じ考え方で、判定そのものは全て Prolog 側にある。
// このパッケージは Prolog に問い合わせるための橋渡しだけを受け持ち、
// 「出せるかどうか」「革命になるか」といった判断を Go では書かない。
//
//	daifugo.pl       … 基本ルール (カードの強さ、役の形、勝敗、称号)
//	daifugo_rules.pl … ローカルルール (革命・8流し・縛り・反則上がり …)
//	daifugo_ai.pl    … CPU の思考 (合法手からランダムに選ぶ)
//
// この 3 つの前に internal/plog の compat.pl が読み込まれる。
package daifugo

import (
	"embed"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ichiban/prolog/engine"

	"hearts/internal/cards"
	"hearts/internal/plog"
)

//go:embed daifugo.pl daifugo_rules.pl daifugo_ai.pl
var prologFS embed.FS

var prologFiles = []string{"daifugo.pl", "daifugo_rules.pl", "daifugo_ai.pl"}

// Card は Prolog の card(Suit, Rank)。ジョーカーは card(joker, jk1/jk2)。
type Card = cards.Card

// Joker1 と Joker2 はデッキに入るジョーカー。
var (
	Joker1 = Card{Suit: cards.JokerSuit, Rank: "jk1"}
	Joker2 = Card{Suit: cards.JokerSuit, Rank: "jk2"}
)

// =============================================
// 場 (フィールド)
// =============================================

// Field は場に出ているカードの状態。Prolog の
// field(種別, 枚数, 強さ, マーク, ジョーカー数, 縛りマーク, 激縛り) に対応する。
// Empty が真なら Prolog 側では nil (場が空、つまり親番) になる。
type Field struct {
	Empty    bool
	Kind     string   // "group" (同じ数字) または "seq" (階段)
	Len      int      // 枚数
	Strength int      // 強さ。ジョーカーだけの出し方は 99
	Suits    []string // 使われたマーク (ジョーカーを除く)
	Jokers   int      // 含まれるジョーカーの枚数
	Bound    []string // 縛られているマーク
	Strict   bool     // 激縛り中か
}

// EmptyField は場が空の状態。
func EmptyField() Field { return Field{Empty: true} }

// Term は Prolog の項テキストを返す。
func (f Field) Term() string {
	if f.Empty {
		return "nil"
	}
	return fmt.Sprintf("field(%s,%d,%d,%s,%d,%s,%s)",
		f.Kind, f.Len, f.Strength,
		plog.Atoms(f.Suits), f.Jokers, plog.Atoms(f.Bound), plog.Atom(f.Strict))
}

var (
	atomField = engine.NewAtom("field")
	atomNil   = engine.NewAtom("nil")
	atomTrue  = engine.NewAtom("true")
)

// Scan は prolog.Scanner を実装する。
func (f *Field) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	t := env.Resolve(term)
	if a, ok := t.(engine.Atom); ok && a == atomNil {
		*f = EmptyField()
		return nil
	}
	cmp, ok := t.(engine.Compound)
	if !ok || cmp.Functor() != atomField || cmp.Arity() != 7 {
		return fmt.Errorf("field/7 ではない項です: %v", t)
	}

	get := func(i int) (string, error) {
		s, ok := cards.AtomOrIntText(cmp.Arg(i), env)
		if !ok {
			return "", fmt.Errorf("field/7 の第 %d 引数を解釈できません", i+1)
		}
		return s, nil
	}
	atoms := func(i int) ([]string, error) {
		var out []string
		iter := engine.ListIterator{List: env.Resolve(cmp.Arg(i)), Env: env}
		for iter.Next() {
			s, ok := cards.AtomOrIntText(iter.Current(), env)
			if !ok {
				return nil, fmt.Errorf("field/7 の第 %d 引数のリストを解釈できません", i+1)
			}
			out = append(out, s)
		}
		return out, iter.Err()
	}

	kind, err := get(0)
	if err != nil {
		return err
	}
	length, err := get(1)
	if err != nil {
		return err
	}
	strength, err := get(2)
	if err != nil {
		return err
	}
	suits, err := atoms(3)
	if err != nil {
		return err
	}
	jokers, err := get(4)
	if err != nil {
		return err
	}
	bound, err := atoms(5)
	if err != nil {
		return err
	}

	n1, err := strconv.Atoi(length)
	if err != nil {
		return err
	}
	n2, err := strconv.Atoi(strength)
	if err != nil {
		return err
	}
	n3, err := strconv.Atoi(jokers)
	if err != nil {
		return err
	}

	strict := false
	if a, ok := env.Resolve(cmp.Arg(6)).(engine.Atom); ok && a == atomTrue {
		strict = true
	}

	*f = Field{
		Kind: kind, Len: n1, Strength: n2,
		Suits: suits, Jokers: n3, Bound: bound, Strict: strict,
	}
	return nil
}

// =============================================
// 出したカードの効果
// =============================================

// Effects は 1 手が場に与える効果。Prolog の effects/3 に対応する。
type Effects struct {
	Cut        bool // 8流し・スペ3返しで場が流れる
	Revolution bool // 革命の切り替えが起きる
	JackBack   bool // Jバックが発動する
}

var atomEffects = engine.NewAtom("effects")

// Scan は prolog.Scanner を実装する。
func (e *Effects) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	cmp, ok := env.Resolve(term).(engine.Compound)
	if !ok || cmp.Functor() != atomEffects || cmp.Arity() != 3 {
		return fmt.Errorf("effects/3 ではない項です: %v", env.Resolve(term))
	}
	isTrue := func(i int) bool {
		a, ok := env.Resolve(cmp.Arg(i)).(engine.Atom)
		return ok && a == atomTrue
	}
	e.Cut, e.Revolution, e.JackBack = isTrue(0), isTrue(1), isTrue(2)
	return nil
}

// =============================================
// Prolog エンジン
// =============================================

// Engine は Prolog インタプリタを包み、大富豪に必要な問い合わせを提供する。
// 並行安全ではない。対局ごとに 1 つ作ること。
type Engine struct {
	p *plog.Interp
}

// New は Prolog を起動し、ルールファイル群を読み込む。
// dir が空なら実行ファイルに埋め込んだものを、そうでなければそのディレクトリの
// .pl を読む。
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

// --- デッキと配札 ---

// Deck は daifugo_rules.pl の deck/1。
func (e *Engine) Deck() ([]Card, error) {
	var v struct{ Deck []Card }
	err := e.p.Scan("deck(Deck).", &v)
	return v.Deck, err
}

// DealSizes は総枚数を人数で分けたときの各自の枚数。
func (e *Engine) DealSizes(players, total int) ([]int, error) {
	var v struct{ Sizes []int }
	err := e.p.Scan(fmt.Sprintf("deal_sizes(%d, %d, Sizes).", players, total), &v)
	return v.Sizes, err
}

// ValidateDeal は配られた状態が正当かを daifugo.pl の validate_deal/2 で確かめる。
func (e *Engine) ValidateDeal(hands [][]Card, deck []Card) (bool, error) {
	return e.p.Prove(fmt.Sprintf("validate_deal(%s, %s).",
		cards.ListOfLists(hands), cards.List(deck)))
}

// OpeningCard はゲーム開始カード (♦3)。
func (e *Engine) OpeningCard() (Card, error) {
	var v struct{ C Card }
	err := e.p.Scan("opening_card(C).", &v)
	return v.C, err
}

// --- 手 ---

// ResolvePlay は 1 手が出せるかを判定し、出せるなら新しい場と効果を返す。
// 出せない場合は ok が false になる (誤りではない)。
func (e *Engine) ResolvePlay(hand, play []Card, field Field, rev bool) (Field, Effects, bool, error) {
	q := fmt.Sprintf("resolve_play(%s, %s, %s, %s, NewField, Fx).",
		cards.List(hand), cards.List(play), field.Term(), plog.Atom(rev))
	var v struct {
		NewField Field
		Fx       Effects
	}
	err := e.p.Scan(q, &v)
	switch {
	case err == nil:
		return v.NewField, v.Fx, true, nil
	case errors.Is(err, plog.ErrNoSolution):
		return Field{}, Effects{}, false, nil
	default:
		return Field{}, Effects{}, false, err
	}
}

// LegalPlays は今の場に対して出せる手を全て返す。
//
// 候補の作り方は daifugo_ai.pl の candidate/2 に従う。
// 「持っている札をあえてジョーカーで置き換える」ような手は列挙されないが、
// 人間が選んだ手は ResolvePlay が個別に判定するので取りこぼしにはならない。
func (e *Engine) LegalPlays(hand []Card, field Field, rev bool) ([][]Card, error) {
	q := fmt.Sprintf("legal_plays(%s, %s, %s, Plays).",
		cards.List(hand), field.Term(), plog.Atom(rev))
	var v struct{ Plays [][]Card }
	err := e.p.Scan(q, &v)
	return v.Plays, err
}

// ChoosePlay は CPU の選択。合法手 (とパス) から seed に基づいて 1 つ選ぶ。
// pass が真ならパスを選んだということ。
func (e *Engine) ChoosePlay(hand []Card, field Field, rev, canPass bool, seed int) ([]Card, bool, error) {
	q := fmt.Sprintf("choose_play(%s, %s, %s, %s, %d, Choice).",
		cards.List(hand), field.Term(), plog.Atom(rev), plog.Atom(canPass), seed)
	var v struct{ Choice choice }
	err := e.p.Scan(q, &v)
	if errors.Is(err, plog.ErrNoSolution) {
		return nil, true, nil // 出せる手が無ければパス
	}
	if err != nil {
		return nil, false, err
	}
	return v.Choice.cards, v.Choice.pass, nil
}

// choice は Prolog の pass / play(カードのリスト) を受ける。
type choice struct {
	pass  bool
	cards []Card
}

var (
	atomPass = engine.NewAtom("pass")
	atomPlay = engine.NewAtom("play")
)

func (c *choice) Scan(vm *engine.VM, term engine.Term, env *engine.Env) error {
	t := env.Resolve(term)
	if a, ok := t.(engine.Atom); ok && a == atomPass {
		c.pass, c.cards = true, nil
		return nil
	}
	cmp, ok := t.(engine.Compound)
	if !ok || cmp.Functor() != atomPlay || cmp.Arity() != 1 {
		return fmt.Errorf("pass でも play/1 でもない項です: %v", t)
	}
	c.pass = false
	c.cards = nil
	iter := engine.ListIterator{List: env.Resolve(cmp.Arg(0)), Env: env}
	for iter.Next() {
		var card Card
		if err := card.Scan(vm, iter.Current(), env); err != nil {
			return err
		}
		c.cards = append(c.cards, card)
	}
	return iter.Err()
}

// FirstLeadOK は 1 ゲーム目の最初の一手として許される手か (♦3 ルール)。
func (e *Engine) FirstLeadOK(hand, play []Card) (bool, error) {
	return e.p.Prove(fmt.Sprintf("first_lead_ok(%s, %s).",
		cards.List(hand), cards.List(play)))
}

// FoulFinish は最後の一手が反則上がりに当たるか。
func (e *Engine) FoulFinish(play []Card, rev bool) (bool, error) {
	return e.p.Prove(fmt.Sprintf("foul_finish(%s, %s).",
		cards.List(play), plog.Atom(rev)))
}

// FoulShortage は 10捨て・7渡しで「手札がマイナスになる」反則上がりか。
func (e *Engine) FoulShortage(required, remaining int) (bool, error) {
	return e.p.Prove(fmt.Sprintf("foul_shortage(%d, %d).", required, remaining))
}

// --- 出したカードに付いてくる処理 ---

// TenDiscardCount は 10捨てで捨てる枚数。使わない設定なら 0。
func (e *Engine) TenDiscardCount(play []Card) (int, error) {
	var v struct{ N int }
	err := e.p.Scan(fmt.Sprintf("ten_discard_count(%s, N).", cards.List(play)), &v)
	return v.N, err
}

// SevenGiveCount は 7渡しで渡す枚数。使わない設定なら 0。
func (e *Engine) SevenGiveCount(play []Card) (int, error) {
	var v struct{ N int }
	err := e.p.Scan(fmt.Sprintf("seven_give_count(%s, N).", cards.List(play)), &v)
	return v.N, err
}

// JackBackOptional は Jバックを「出した人が決める」設定か。
func (e *Engine) JackBackOptional() (bool, error) {
	return e.p.Prove("jack_back_optional.")
}

// --- 表示のための問い合わせ ---

// CardStrength はカード 1 枚の強さ。表示順の並べ替えに使う。
func (e *Engine) CardStrength(c Card, rev bool) (int, error) {
	var v struct{ V int }
	err := e.p.Scan(fmt.Sprintf("card_strength(%s, %s, V).", c.Term(), plog.Atom(rev)), &v)
	return v.V, err
}

// StrengthRank は強さの値に対応するランク名を返す。
// 場に出ている札を「9 のペア」のように書くために使う。
func (e *Engine) StrengthRank(strength int, rev bool) (string, error) {
	// ランクは 10 までが整数、j/q/k/a がアトムなので両方を受ける。
	var v struct{ R cards.Text }
	err := e.p.Scan(fmt.Sprintf("strength_rank(%d, %s, R).", strength, plog.Atom(rev)), &v)
	if errors.Is(err, plog.ErrNoSolution) {
		return "", nil
	}
	return string(v.R), err
}

// --- 称号・交換・得点 ---

// Titles は人数に応じた称号を 1 位から順に返す。
func (e *Engine) Titles(players int) ([]string, error) {
	var v struct{ Titles []string }
	err := e.p.Scan(fmt.Sprintf("titles(%d, Titles).", players), &v)
	return v.Titles, err
}

// Exchange はゲーム開始時のカード交換 1 組。順位は 1 始まり。
type Exchange struct {
	High  int // 上位のプレイヤーの順位 (好きなカードを渡す)
	Low   int // 下位のプレイヤーの順位 (強いカードを渡す)
	Count int // 枚数
}

// Exchanges は人数に応じたカード交換の組み合わせ。
func (e *Engine) Exchanges(players int) ([]Exchange, error) {
	q := fmt.Sprintf("exchange_list(%d, Highs, Lows, Counts).", players)
	var v struct {
		Highs  []int
		Lows   []int
		Counts []int
	}
	if err := e.p.Scan(q, &v); err != nil {
		return nil, err
	}
	if len(v.Highs) != len(v.Lows) || len(v.Highs) != len(v.Counts) {
		return nil, fmt.Errorf("exchange_list/4 の結果の長さが揃っていません")
	}
	out := make([]Exchange, len(v.Highs))
	for i := range v.Highs {
		out[i] = Exchange{High: v.Highs[i], Low: v.Lows[i], Count: v.Counts[i]}
	}
	return out, nil
}

// RankPoints は順位ごとの得点 (1 位から順)。
func (e *Engine) RankPoints(players int) ([]int, error) {
	var v struct{ Points []int }
	err := e.p.Scan(fmt.Sprintf("rank_point_list(%d, Points).", players), &v)
	return v.Points, err
}

// StrongestCards は手札から強い順に count 枚。下位から上位への献上に使う。
func (e *Engine) StrongestCards(hand []Card, count int) ([]Card, error) {
	var v struct{ Cards []Card }
	err := e.p.Scan(fmt.Sprintf("strongest_cards(%s, %d, Cards).", cards.List(hand), count), &v)
	return v.Cards, err
}

// WeakestCards は手札から弱い順に count 枚。上位から下位への下げ渡しに使う。
func (e *Engine) WeakestCards(hand []Card, count int) ([]Card, error) {
	var v struct{ Cards []Card }
	err := e.p.Scan(fmt.Sprintf("weakest_cards(%s, %d, Cards).", cards.List(hand), count), &v)
	return v.Cards, err
}

// FieldLabel は場に出ている手を「♠9 のペア」のように書き表す。
func (e *Engine) FieldLabel(f Field, rev bool) (string, error) {
	if f.Empty {
		return "場は空", nil
	}
	rank, err := e.StrengthRank(f.Strength, rev)
	if err != nil {
		return "", err
	}
	label := cards.RankLabel(rank)
	if rank == "joker" || f.Strength == 99 {
		label = "JOKER"
	}
	if f.Kind == "seq" {
		return fmt.Sprintf("%s までの %d 枚階段", label, f.Len), nil
	}
	switch f.Len {
	case 1:
		return label + " 1枚", nil
	case 2:
		return label + " のペア", nil
	default:
		return fmt.Sprintf("%s の %d 枚", label, f.Len), nil
	}
}

// SuitsLabel は縛りの表示用に ♠♥ のような並びを作る。
func SuitsLabel(suits []string) string {
	var b strings.Builder
	for _, s := range suits {
		b.WriteString(cards.SuitSymbol(s))
	}
	return b.String()
}
