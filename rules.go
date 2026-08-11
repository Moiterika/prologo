package main

// rules.go — Prolog ブリッジ。
//
// ゲームのルール判定は「すべて」Prolog 側 (hearts.pl とその上のレイヤ) に
// 問い合わせる。Go 側はシャッフル・入出力・進行管理だけを担当する。

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

// Rules は Prolog インタプリタを包み、ゲームが必要とする問い合わせを提供する。
type Rules struct {
	p          *prolog.Interpreter
	rankValues map[string]int // 表示用に hearts.pl の rank_value/2 を取り込んだもの
}

// NewRules は Prolog を起動し、ルールファイル群を読み込む。
// dir が空なら実行ファイルに埋め込んだものを、そうでなければそのディレクトリの
// .pl を読む (hearts.pl を書き換えて挙動を試したいとき用)。
func NewRules(dir string) (*Rules, error) {
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

	r := &Rules{p: p, rankValues: map[string]int{}}
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
func (r *Rules) scan(query string, dest interface{}) error {
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
func (r *Rules) prove(query string) (bool, error) {
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
func (r *Rules) TotalPlayers() (int, error) {
	var v struct{ N int }
	err := r.scan("total_players(N).", &v)
	return v.N, err
}

// HandSize は hearts.pl の hand_size/1。
func (r *Rules) HandSize() (int, error) {
	var v struct{ S int }
	err := r.scan("hand_size(S).", &v)
	return v.S, err
}

// SetPlayers は hearts.pl の set_players/1。
func (r *Rules) SetPlayers(n int) error {
	ok, err := r.prove(fmt.Sprintf("set_players(%d).", n))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%d 人はサポートされていません (hearts.pl の game_config/4 を参照)", n)
	}
	return nil
}

// Deck は hearts_rules.pl の deck/1 (hearts.pl の suit/1, rank/1,
// excluded_cards/1 から生成される)。
func (r *Rules) Deck() ([]Card, error) {
	var v struct{ Deck []Card }
	err := r.scan("deck(Deck).", &v)
	return v.Deck, err
}

func (r *Rules) rankValue(rank string) (int, error) {
	var v struct{ V int }
	err := r.scan(fmt.Sprintf("rank_value(%s, V).", rank), &v)
	return v.V, err
}

// RankValue は表示用の並べ替えに使う (hearts.pl の rank_value/2 由来)。
func (r *Rules) RankValue(rank string) int {
	return r.rankValues[rank]
}

// --- 配札 ---

// ValidateDeal は hearts.pl の validate_deal/1。
func (r *Rules) ValidateDeal(hands [][]Card) (bool, error) {
	return r.prove(fmt.Sprintf("validate_deal(%s).", cardListOfLists(hands)))
}

// --- 手番 ---

// OpeningCard は第1トリックで必ず出すカード (通常 2♣)。
func (r *Rules) OpeningCard() (Card, error) {
	var v struct{ C Card }
	err := r.scan("opening_card(C).", &v)
	return v.C, err
}

// LegalCards は hearts.pl の can_play/3 に、ハートブレイクと第1トリックの
// 制約を重ねた legal_cards/5 の結果。
func (r *Rules) LegalCards(hand []Card, lead *Card, trickNo int, broken bool) ([]Card, error) {
	q := fmt.Sprintf("legal_cards(%s, %s, %d, %s, Cards).",
		cardList(hand), leadTerm(lead), trickNo, boolAtom(broken))
	var v struct{ Cards []Card }
	err := r.scan(q, &v)
	return v.Cards, err
}

// IsLegal は 1 枚のカードが出せるかを legal_play/5 で確かめる。
func (r *Rules) IsLegal(hand []Card, lead *Card, trickNo int, broken bool, card Card) (bool, error) {
	q := fmt.Sprintf("legal_play(%s, %s, %d, %s, %s).",
		cardList(hand), leadTerm(lead), trickNo, boolAtom(broken), card.Term())
	return r.prove(q)
}

// --- トリック ---

// DetermineWinner は hearts.pl の determine_winner/3。
func (r *Rules) DetermineWinner(trick []Card, leadSuit string) (Card, error) {
	q := fmt.Sprintf("determine_winner(%s, %s, W).", cardList(trick), leadSuit)
	var v struct{ W Card }
	err := r.scan(q, &v)
	return v.W, err
}

// ContainsHeart はハートブレイクの判定に使う。
func (r *Rules) ContainsHeart(cards []Card) (bool, error) {
	return r.prove(fmt.Sprintf("contains_heart(%s).", cardList(cards)))
}

// CardPoint は hearts.pl の card_point/2。
func (r *Rules) CardPoint(c Card) (int, error) {
	var v struct{ P int }
	err := r.scan(fmt.Sprintf("card_point(%s, P).", c.Term()), &v)
	return v.P, err
}

// CalculateScore は hearts.pl の calculate_score/2。
func (r *Rules) CalculateScore(cards []Card) (int, error) {
	var v struct{ T int }
	err := r.scan(fmt.Sprintf("calculate_score(%s, T).", cardList(cards)), &v)
	return v.T, err
}

// --- パス ---

// PassCount は 1 回に交換する枚数。
func (r *Rules) PassCount() (int, error) {
	var v struct{ N int }
	err := r.scan("pass_count(N).", &v)
	return v.N, err
}

// PassDirection はラウンド番号から交換方向を返す (left/right/across/hold)。
func (r *Rules) PassDirection(round int) (string, error) {
	var v struct{ D string }
	err := r.scan(fmt.Sprintf("pass_direction(%d, D).", round), &v)
	return v.D, err
}

// PassTarget は from が誰に渡すかを返す。
func (r *Rules) PassTarget(from int, dir string, n int) (int, error) {
	var v struct{ To int }
	err := r.scan(fmt.Sprintf("pass_target(%d, %s, %d, To).", from, dir, n), &v)
	return v.To, err
}

// --- 精算 ---

// RoundScores は取ったカード束から各自の失点を求める。
// スラム (1人で全失点を独占) の達成者がいればそのプレイヤー番号、
// いなければ -1 を返す。
func (r *Rules) RoundScores(taken [][]Card) ([]int, int, error) {
	q := fmt.Sprintf("round_scores(%s, Final, Slammer).", cardListOfLists(taken))
	var v struct {
		Final   []int
		Slammer int
	}
	err := r.scan(q, &v)
	return v.Final, v.Slammer, err
}

// TotalRoundPoints は 1 ラウンドに存在する失点の総和 (通常 26)。
func (r *Rules) TotalRoundPoints() (int, error) {
	var v struct{ Total int }
	err := r.scan("total_round_points(Total).", &v)
	return v.Total, err
}

// GameOver は誰かが上限点に達したか。
func (r *Rules) GameOver(scores []int, limit int) (bool, error) {
	return r.prove(fmt.Sprintf("game_over(%s, %d).", intList(scores), limit))
}

// --- CPU の思考 ---

// ChooseCard は hearts_ai.pl の choose_card/6。
func (r *Rules) ChooseCard(hand []Card, lead *Card, trick []Card, trickNo int, broken bool) (Card, error) {
	q := fmt.Sprintf("choose_card(%s, %s, %s, %d, %s, C).",
		cardList(hand), leadTerm(lead), cardList(trick), trickNo, boolAtom(broken))
	var v struct{ C Card }
	err := r.scan(q, &v)
	return v.C, err
}

// ChoosePass は hearts_ai.pl の choose_pass/3。
func (r *Rules) ChoosePass(hand []Card, count int) ([]Card, error) {
	q := fmt.Sprintf("choose_pass(%s, %d, Cards).", cardList(hand), count)
	var v struct{ Cards []Card }
	err := r.scan(q, &v)
	return v.Cards, err
}
