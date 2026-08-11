// Package rules はハートのルール判定を提供する。
//
// 判定そのものは「すべて」Prolog で書かれている。このパッケージは純 Go 実装の
// Prolog 処理系 (github.com/ichiban/prolog) にルールファイルを読み込ませ、
// Go から問い合わせるための薄い橋渡しをするだけで、ルールを Go で書き直しては
// いない。
//
//	compat.pl       … SWI 固有述語の互換レイヤ (hearts.pl を無修正で読むため)
//	hearts.pl       … 元のルール定義 (変更なし)
//	hearts_rules.pl … ハートブレイク・第1トリック・パス・精算などの追加規則
//	hearts_ai.pl    … CPU の思考
//
// 使い方は Example を参照。
package rules

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ichiban/prolog"
	"github.com/ichiban/prolog/engine"
)

//go:embed compat.pl hearts.pl hearts_rules.pl hearts_ai.pl
var prologFS embed.FS

// 読み込み順序。compat.pl が SWI 固有の述語を補い、hearts.pl は無修正で載る。
var prologFiles = []string{"compat.pl", "hearts.pl", "hearts_rules.pl", "hearts_ai.pl"}

// =============================================
// カード
// =============================================

// Card は Prolog の card(Suit, Rank) に対応する。
// Suit は "s"/"h"/"d"/"c"、Rank は "2".."10","j","q","k","a"。
type Card struct {
	Suit string
	Rank string
}

// Term は Prolog の項テキストを返す。スート・ランクは固定語彙なので
// そのまま埋め込んでよい。
func (c Card) Term() string {
	return "card(" + c.Suit + "," + c.Rank + ")"
}

var atomCard = engine.NewAtom("card")

// Scan は prolog.Scanner を実装し、card(S, R) を Card に変換する。
// ランクは 2..10 が整数、j/q/k/a がアトムなので両方を受ける。
func (c *Card) Scan(_ *engine.VM, term engine.Term, env *engine.Env) error {
	t := env.Resolve(term)
	cmp, ok := t.(engine.Compound)
	if !ok || cmp.Functor() != atomCard || cmp.Arity() != 2 {
		return fmt.Errorf("card/2 ではない項です: %v", t)
	}
	suit, ok := atomOrIntText(cmp.Arg(0), env)
	if !ok {
		return fmt.Errorf("スートを解釈できません: %v", cmp.Arg(0))
	}
	rank, ok := atomOrIntText(cmp.Arg(1), env)
	if !ok {
		return fmt.Errorf("ランクを解釈できません: %v", cmp.Arg(1))
	}
	c.Suit, c.Rank = suit, rank
	return nil
}

func atomOrIntText(term engine.Term, env *engine.Env) (string, bool) {
	switch v := env.Resolve(term).(type) {
	case engine.Atom:
		return v.String(), true
	case engine.Integer:
		return strconv.Itoa(int(v)), true
	default:
		return "", false
	}
}

// cardList は Go のカード列を Prolog のリスト項に変換する。
func cardList(cards []Card) string {
	terms := make([]string, len(cards))
	for i, c := range cards {
		terms[i] = c.Term()
	}
	return "[" + strings.Join(terms, ",") + "]"
}

// Remove は手札から 1 枚を取り除いた新しいスライスを返す。
// 元のスライスは変更しない。同じカードが無ければそのまま返す。
func Remove(hand []Card, card Card) []Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}

// IndexOf は cards の中で target が何番目かを返す。無ければ -1。
func IndexOf(cards []Card, target Card) int {
	for i, c := range cards {
		if c == target {
			return i
		}
	}
	return -1
}

func cardListOfLists(hands [][]Card) string {
	terms := make([]string, len(hands))
	for i, h := range hands {
		terms[i] = cardList(h)
	}
	return "[" + strings.Join(terms, ",") + "]"
}

func intList(xs []int) string {
	terms := make([]string, len(xs))
	for i, x := range xs {
		terms[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(terms, ",") + "]"
}

// leadTerm は台札を表す項。リード(台札なし)は nil。
func leadTerm(lead *Card) string {
	if lead == nil {
		return "nil"
	}
	return lead.Term()
}

func boolAtom(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// =============================================
// Prolog エンジン
// =============================================

// Engine は Prolog インタプリタを包み、ゲームが必要とする問い合わせを提供する。
//
// Engine は並行安全ではない。同時に複数の対局を進める場合は、対局ごとに
// 1 つずつ作ること。
type Engine struct {
	p          *prolog.Interpreter
	rankValues map[string]int // 表示用に hearts.pl の rank_value/2 を取り込んだもの
}

// New は Prolog を起動し、ルールファイル群を読み込む。
// dir が空なら実行ファイルに埋め込んだものを、そうでなければそのディレクトリの
// .pl を読む (hearts.pl を書き換えて挙動を試したいとき用)。
func New(dir string) (*Engine, error) {
	// Prolog 側に標準入力は与えない (端末入力は Go の UI が独占する)。
	p := prolog.New(strings.NewReader(""), os.Stdout)

	for _, name := range prologFiles {
		var (
			src []byte
			err error
		)
		if dir == "" {
			src, err = prologFS.ReadFile(name)
		} else {
			src, err = os.ReadFile(filepath.Join(dir, name))
		}
		if err != nil {
			return nil, fmt.Errorf("%s の読み込みに失敗しました: %w", name, err)
		}
		if err := p.Exec(string(src)); err != nil {
			return nil, fmt.Errorf("%s の解釈に失敗しました: %w", name, err)
		}
	}

	r := &Engine{p: p, rankValues: map[string]int{}}
	deck, err := r.Deck()
	if err != nil {
		return nil, err
	}
	for _, c := range deck {
		if _, ok := r.rankValues[c.Rank]; ok {
			continue
		}
		v, err := r.rankValue(c.Rank)
		if err != nil {
			return nil, err
		}
		r.rankValues[c.Rank] = v
	}
	return r, nil
}

// scan は最初の解を dest に取り込む。解が無い場合は errNoSolution を返す。
func (r *Engine) scan(query string, dest interface{}) error {
	sol := r.p.QuerySolution(query)
	if err := sol.Err(); err != nil {
		if errors.Is(err, prolog.ErrNoSolutions) {
			return errNoSolution
		}
		return fmt.Errorf("問い合わせ %s に失敗しました: %w", query, err)
	}
	if dest == nil {
		return nil
	}
	return sol.Scan(dest)
}

// prove はゴールが成功するかどうかだけを判定する。
func (r *Engine) prove(query string) (bool, error) {
	err := r.scan(query, nil)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errNoSolution):
		return false, nil
	default:
		return false, err
	}
}

var errNoSolution = errors.New("解がありません")

// --- hearts.pl の設定値 ---

// TotalPlayers は hearts.pl の total_players/1。
func (r *Engine) TotalPlayers() (int, error) {
	var v struct{ N int }
	err := r.scan("total_players(N).", &v)
	return v.N, err
}

// HandSize は hearts.pl の hand_size/1。
func (r *Engine) HandSize() (int, error) {
	var v struct{ S int }
	err := r.scan("hand_size(S).", &v)
	return v.S, err
}

// SetPlayers は hearts.pl の set_players/1。
func (r *Engine) SetPlayers(n int) error {
	ok, err := r.prove(fmt.Sprintf("set_players(%d).", n))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%d 人はサポートされていません (hearts.pl の game_config/4 を参照)", n)
	}
	return nil
}

// --- 第1トリックの流儀 ---

// FirstTrickPoints は第1トリックの点数札の扱い。
type FirstTrickPoints string

const (
	// ForbidAllPoints は点数札 (ハート全般と ♠Q) を出せない流儀。
	// いわゆる no blood on the first trick。
	ForbidAllPoints FirstTrickPoints = "forbid_all"
	// ForbidHearts はハートだけ出せない流儀 (♠Q は出せる)。
	ForbidHearts FirstTrickPoints = "forbid_hearts"
	// AllowPoints は第1トリックの点数札制限なし。
	AllowPoints FirstTrickPoints = "allow"
)

// FirstTrickLead は第1トリックのリードの決め方。
type FirstTrickLead string

const (
	// LeadOpeningCard は開始カード (♣2) から始める流儀。
	LeadOpeningCard FirstTrickLead = "opening_card"
	// LeadFree は ♣2 の持ち主が先手だが出すカードは自由、という流儀。
	LeadFree FirstTrickLead = "free"
)

// SetFirstTrickPoints は第1トリックの点数札の扱いを切り替える。
func (r *Engine) SetFirstTrickPoints(mode FirstTrickPoints) error {
	ok, err := r.prove(fmt.Sprintf("set_first_trick_points(%s).", mode))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("第1トリックの点数札の指定が不正です: %q", mode)
	}
	return nil
}

// SetFirstTrickLead は第1トリックのリードの決め方を切り替える。
func (r *Engine) SetFirstTrickLead(mode FirstTrickLead) error {
	ok, err := r.prove(fmt.Sprintf("set_first_trick_lead(%s).", mode))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("第1トリックのリードの指定が不正です: %q", mode)
	}
	return nil
}

// HeartsBreakRule はハートブレイク規則を使うかどうか。
type HeartsBreakRule string

const (
	// HeartsBreakOn は得点札 (♥ 全般と ♠Q) が場に出るまで
	// ハートでリードできない流儀。
	HeartsBreakOn HeartsBreakRule = "on"
	// HeartsBreakOff はいつでもハートでリードできる流儀 (既定)。
	HeartsBreakOff HeartsBreakRule = "off"
)

// SetHeartsBreakRule はハートブレイク規則の有無を切り替える。
func (r *Engine) SetHeartsBreakRule(mode HeartsBreakRule) error {
	ok, err := r.prove(fmt.Sprintf("set_hearts_break_rule(%s).", mode))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("ハートブレイク規則の指定が不正です: %q", mode)
	}
	return nil
}

// Deck は hearts_rules.pl の deck/1 (hearts.pl の suit/1, rank/1,
// excluded_cards/1 から生成される)。
func (r *Engine) Deck() ([]Card, error) {
	var v struct{ Deck []Card }
	err := r.scan("deck(Deck).", &v)
	return v.Deck, err
}

func (r *Engine) rankValue(rank string) (int, error) {
	var v struct{ V int }
	err := r.scan(fmt.Sprintf("rank_value(%s, V).", rank), &v)
	return v.V, err
}

// RankValue は表示用の並べ替えに使う (hearts.pl の rank_value/2 由来)。
func (r *Engine) RankValue(rank string) int {
	return r.rankValues[rank]
}

// --- 配札 ---

// ValidateDeal は hearts.pl の validate_deal/1。
func (r *Engine) ValidateDeal(hands [][]Card) (bool, error) {
	return r.prove(fmt.Sprintf("validate_deal(%s).", cardListOfLists(hands)))
}

// --- 手番 ---

// OpeningCard は第1トリックで必ず出すカード (通常 2♣)。
func (r *Engine) OpeningCard() (Card, error) {
	var v struct{ C Card }
	err := r.scan("opening_card(C).", &v)
	return v.C, err
}

// LegalCards は hearts.pl の can_play/3 に、ハートブレイクと第1トリックの
// 制約を重ねた legal_cards/5 の結果。
func (r *Engine) LegalCards(hand []Card, lead *Card, trickNo int, broken bool) ([]Card, error) {
	q := fmt.Sprintf("legal_cards(%s, %s, %d, %s, Cards).",
		cardList(hand), leadTerm(lead), trickNo, boolAtom(broken))
	var v struct{ Cards []Card }
	err := r.scan(q, &v)
	return v.Cards, err
}

// IsLegal は 1 枚のカードが出せるかを legal_play/5 で確かめる。
func (r *Engine) IsLegal(hand []Card, lead *Card, trickNo int, broken bool, card Card) (bool, error) {
	q := fmt.Sprintf("legal_play(%s, %s, %d, %s, %s).",
		cardList(hand), leadTerm(lead), trickNo, boolAtom(broken), card.Term())
	return r.prove(q)
}

// --- トリック ---

// DetermineWinner は hearts.pl の determine_winner/3。
func (r *Engine) DetermineWinner(trick []Card, leadSuit string) (Card, error) {
	q := fmt.Sprintf("determine_winner(%s, %s, W).", cardList(trick), leadSuit)
	var v struct{ W Card }
	err := r.scan(q, &v)
	return v.W, err
}

// ContainsPointCard は得点札 (ハート全般と ♠Q) が含まれるか。
// ハートブレイクの判定に使う。
func (r *Engine) ContainsPointCard(cards []Card) (bool, error) {
	return r.prove(fmt.Sprintf("contains_point_card(%s).", cardList(cards)))
}

// CardPoint は hearts.pl の card_point/2。
func (r *Engine) CardPoint(c Card) (int, error) {
	var v struct{ P int }
	err := r.scan(fmt.Sprintf("card_point(%s, P).", c.Term()), &v)
	return v.P, err
}

// CalculateScore は hearts.pl の calculate_score/2。
func (r *Engine) CalculateScore(cards []Card) (int, error) {
	var v struct{ T int }
	err := r.scan(fmt.Sprintf("calculate_score(%s, T).", cardList(cards)), &v)
	return v.T, err
}

// --- パス ---

// PassCount は 1 回に交換する枚数。
func (r *Engine) PassCount() (int, error) {
	var v struct{ N int }
	err := r.scan("pass_count(N).", &v)
	return v.N, err
}

// PassDirection はラウンド番号から交換方向を返す (left/right/across/hold)。
func (r *Engine) PassDirection(round int) (string, error) {
	var v struct{ D string }
	err := r.scan(fmt.Sprintf("pass_direction(%d, D).", round), &v)
	return v.D, err
}

// PassTarget は from が誰に渡すかを返す。
func (r *Engine) PassTarget(from int, dir string, n int) (int, error) {
	var v struct{ To int }
	err := r.scan(fmt.Sprintf("pass_target(%d, %s, %d, To).", from, dir, n), &v)
	return v.To, err
}

// --- 精算 ---

// RoundScores は取ったカード束から各自の失点を求める。
// スラム (1人で全失点を独占) の達成者がいればそのプレイヤー番号、
// いなければ -1 を返す。
func (r *Engine) RoundScores(taken [][]Card) ([]int, int, error) {
	q := fmt.Sprintf("round_scores(%s, Final, Slammer).", cardListOfLists(taken))
	var v struct {
		Final   []int
		Slammer int
	}
	err := r.scan(q, &v)
	return v.Final, v.Slammer, err
}

// TotalRoundPoints は 1 ラウンドに存在する失点の総和 (通常 26)。
func (r *Engine) TotalRoundPoints() (int, error) {
	var v struct{ Total int }
	err := r.scan("total_round_points(Total).", &v)
	return v.Total, err
}

// GameOver は誰かが上限点に達したか。
func (r *Engine) GameOver(scores []int, limit int) (bool, error) {
	return r.prove(fmt.Sprintf("game_over(%s, %d).", intList(scores), limit))
}

// --- CPU の思考 ---

// ChooseCard は hearts_ai.pl の choose_card/6。
func (r *Engine) ChooseCard(hand []Card, lead *Card, trick []Card, trickNo int, broken bool) (Card, error) {
	q := fmt.Sprintf("choose_card(%s, %s, %s, %d, %s, C).",
		cardList(hand), leadTerm(lead), cardList(trick), trickNo, boolAtom(broken))
	var v struct{ C Card }
	err := r.scan(q, &v)
	return v.C, err
}

// ChoosePass は hearts_ai.pl の choose_pass/3。
func (r *Engine) ChoosePass(hand []Card, count int) ([]Card, error) {
	q := fmt.Sprintf("choose_pass(%s, %d, Cards).", cardList(hand), count)
	var v struct{ Cards []Card }
	err := r.scan(q, &v)
	return v.Cards, err
}
