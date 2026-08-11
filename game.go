package main

// game.go — ゲーム進行。
//
// ルール判定は一切ここで書かず、すべて Rules 経由で Prolog に尋ねる。
// Go が担当するのはシャッフル・手番の管理・画面表示だけ。

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

// Game は 1 ゲーム分の状態。
type Game struct {
	rules     *Rules
	ui        *UI
	rng       *rand.Rand
	players   int
	handSize  int
	passCount int
	maxPoints int // 1 ラウンドの失点総和 (通常 26)
	names     []string
	scores    []int
	limit     int
	humanSeat int
	auto      bool
}

// errQuit は入力が閉じられたことを表す。
var errQuit = errors.New("入力が終了しました")

// NewGame は Prolog から人数・手札枚数などを取得してゲームを組み立てる。
func NewGame(rules *Rules, ui *UI, rng *rand.Rand, limit int, auto bool) (*Game, error) {
	players, err := rules.TotalPlayers()
	if err != nil {
		return nil, err
	}
	handSize, err := rules.HandSize()
	if err != nil {
		return nil, err
	}
	passCount, err := rules.PassCount()
	if err != nil {
		return nil, err
	}
	maxPoints, err := rules.TotalRoundPoints()
	if err != nil {
		return nil, err
	}

	names := make([]string, players)
	names[0] = "あなた"
	for i := 1; i < players; i++ {
		names[i] = fmt.Sprintf("CPU%d", i)
	}

	return &Game{
		rules:     rules,
		ui:        ui,
		rng:       rng,
		players:   players,
		handSize:  handSize,
		passCount: passCount,
		maxPoints: maxPoints,
		names:     names,
		scores:    make([]int, players),
		limit:     limit,
		humanSeat: 0,
		auto:      auto,
	}, nil
}

// Run はゲーム全体を進行する。
func (g *Game) Run() error {
	if err := g.showIntro(); err != nil {
		return err
	}

	for round := 1; ; round++ {
		if err := g.playRound(round); err != nil {
			return err
		}

		g.ui.showScoreTable(g.names, g.scores, g.limit)

		over, err := g.rules.GameOver(g.scores, g.limit)
		if err != nil {
			return err
		}
		if over {
			break
		}
		g.ui.pause()
	}

	g.showResult()
	return nil
}

func (g *Game) showIntro() error {
	u := g.ui
	u.println()
	u.println(u.paint(ansiBold, "==========  ハーツ (Hearts)  =========="))
	u.println("  ルール判定は Prolog (hearts.pl) が行っています。")
	u.println()
	opening, err := g.rules.OpeningCard()
	if err != nil {
		return err
	}

	u.printf("  ・%d 人でプレイ (あなた + CPU %d 人)\n", g.players, g.players-1)
	u.printf("  ・ハート1枚 = 1点、スペードのQ = 13点。失点は少ないほど良い。\n")
	u.printf("  ・台札と同じスートを持っていたら必ずそれを出す (マストフォロー)\n")
	u.printf("  ・第1トリックは %s スタート。点数札は出せない。\n", u.cardText(opening))
	u.printf("  ・ハートが場に出るまで、ハートからはリードできない。\n")
	u.printf("  ・1人で全 %d 点を取ると「スラム」で他全員が %d 点。\n", g.maxPoints, g.maxPoints)
	u.printf("  ・誰かが %d 点に達したら終了。最少失点の人が勝ち。\n", g.limit)
	u.println()
	g.ui.pause()
	return nil
}

func (g *Game) showResult() {
	u := g.ui
	best := 0
	for i, s := range g.scores {
		if s < g.scores[best] {
			best = i
		}
	}
	u.println(u.paint(ansiBold, "==========  ゲーム終了  =========="))
	if best == g.humanSeat {
		u.printf("  %s\n", u.paint(ansiGreen, "あなたの勝ちです！おめでとうございます。"))
	} else {
		u.printf("  勝者: %s (%d点)\n", g.names[best], g.scores[best])
	}
	u.println()
}

// =============================================
// 1 ラウンド
// =============================================

func (g *Game) playRound(round int) error {
	u := g.ui
	u.println()
	u.println(u.paint(ansiBold, fmt.Sprintf("########## 第 %d ラウンド ##########", round)))

	hands, err := g.deal()
	if err != nil {
		return err
	}

	if err := g.passPhase(round, hands); err != nil {
		return err
	}

	taken, err := g.playTricks(hands)
	if err != nil {
		return err
	}

	// 精算は hearts.pl の calculate_score/2 + スラム判定
	final, slammer, err := g.rules.RoundScores(taken)
	if err != nil {
		return err
	}

	u.println()
	u.println(u.paint(ansiBold, "--- ラウンド結果 ---"))
	if slammer >= 0 {
		u.printf("  %s\n", u.paint(ansiGreen,
			fmt.Sprintf("スラム達成！ %sが全 %d 点を独占しました。",
				g.names[slammer], g.maxPoints)))
	}
	for i := range g.names {
		points, err := g.rules.CalculateScore(taken[i])
		if err != nil {
			return err
		}
		g.scores[i] += final[i]
		u.printf("  %s 獲得 %2d 枚 / 素点 %2d  このラウンド %+3d 点\n",
			u.pad(g.names[i], 8), len(taken[i]), points, final[i])
	}
	return nil
}

// deal はデッキを Prolog から取得してシャッフルし、配る。
// 配り終えた状態は hearts.pl の validate_deal/1 で検証する。
func (g *Game) deal() ([][]Card, error) {
	deck, err := g.rules.Deck()
	if err != nil {
		return nil, err
	}
	g.rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	hands := make([][]Card, g.players)
	for i := 0; i < g.players; i++ {
		hands[i] = append([]Card(nil), deck[i*g.handSize:(i+1)*g.handSize]...)
	}

	ok, err := g.rules.ValidateDeal(hands)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("配札が hearts.pl の validate_deal/1 を通りませんでした")
	}
	return hands, nil
}

// =============================================
// カード交換 (パス)
// =============================================

func (g *Game) passPhase(round int, hands [][]Card) error {
	u := g.ui

	dir, err := g.rules.PassDirection(round)
	if err != nil {
		return err
	}
	if dir == "hold" {
		u.printf("\n今回は %s のラウンドです。\n", u.paint(ansiCyan, "交換なし"))
		return nil
	}

	u.printf("\nカード交換: %s %s\n",
		u.paint(ansiCyan, passDirName[dir]),
		fmt.Sprintf("(%d枚)", g.passCount))

	selected := make([][]Card, g.players)
	for i := 0; i < g.players; i++ {
		if i == g.humanSeat && !g.auto {
			picked, err := g.askHumanPass(hands[i])
			if err != nil {
				return err
			}
			selected[i] = picked
		} else {
			picked, err := g.rules.ChoosePass(hands[i], g.passCount)
			if err != nil {
				return err
			}
			selected[i] = picked
		}
	}

	// 全員同時に渡す
	for i := 0; i < g.players; i++ {
		for _, c := range selected[i] {
			hands[i] = removeCard(hands[i], c)
		}
	}
	for i := 0; i < g.players; i++ {
		to, err := g.rules.PassTarget(i, dir, g.players)
		if err != nil {
			return err
		}
		hands[to] = append(hands[to], selected[i]...)

		if to == g.humanSeat {
			var texts []string
			for _, c := range selected[i] {
				texts = append(texts, strings.TrimSpace(u.cardCell(c)))
			}
			u.printf("  %s から受け取りました: %s\n", g.names[i], strings.Join(texts, " "))
		}
	}
	if !g.auto {
		u.println()
		u.showHand(hands[g.humanSeat], nil)
		g.ui.pause()
	}
	return nil
}

func (g *Game) askHumanPass(hand []Card) ([]Card, error) {
	u := g.ui
	u.println()
	shown := u.showHand(hand, nil)
	nums, err := u.askInts(
		fmt.Sprintf("渡す %d 枚の番号を入力してください (例: 1 5 12) >", g.passCount),
		g.passCount, 1, len(shown))
	if err != nil {
		return nil, errQuit
	}
	picked := make([]Card, 0, len(nums))
	for _, n := range nums {
		picked = append(picked, shown[n-1])
	}
	return picked, nil
}

// =============================================
// トリックプレイ
// =============================================

func (g *Game) playTricks(hands [][]Card) ([][]Card, error) {
	u := g.ui

	taken := make([][]Card, g.players)
	broken := false

	leader, err := g.openingLeader(hands)
	if err != nil {
		return nil, err
	}

	for trickNo := 1; trickNo <= g.handSize; trickNo++ {
		var (
			played  []Card
			players []int
			lead    *Card
		)

		for k := 0; k < g.players; k++ {
			pi := (leader + k) % g.players

			legal, err := g.rules.LegalCards(hands[pi], lead, trickNo, broken)
			if err != nil {
				return nil, err
			}
			if len(legal) == 0 {
				return nil, fmt.Errorf("%s に出せるカードがありません (Prolog の legal_cards/5 が空)", g.names[pi])
			}

			var card Card
			if pi == g.humanSeat && !g.auto {
				u.println()
				u.showTrick(g.names, players, played, trickNo, g.handSize)
				card, err = g.askHumanPlay(hands[pi], legal, lead, trickNo, broken)
				if err != nil {
					return nil, err
				}
			} else {
				card, err = g.rules.ChooseCard(hands[pi], lead, played, trickNo, broken)
				if err != nil {
					return nil, err
				}
			}

			// 念のため Prolog に最終確認 (人間の入力・AI の出力の両方を検査)
			ok, err := g.rules.IsLegal(hands[pi], lead, trickNo, broken, card)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("%s が出そうとした %s は hearts.pl のルールに反します",
					g.names[pi], u.cardText(card))
			}

			hands[pi] = removeCard(hands[pi], card)
			played = append(played, card)
			players = append(players, pi)
			if k == 0 {
				c := card
				lead = &c
			}
		}

		// 勝者判定は hearts.pl の determine_winner/3
		winCard, err := g.rules.DetermineWinner(played, lead.Suit)
		if err != nil {
			return nil, err
		}
		winner := players[indexOfCard(played, winCard)]
		taken[winner] = append(taken[winner], played...)

		justBroke := false
		if !broken {
			broken, err = g.rules.ContainsHeart(played)
			if err != nil {
				return nil, err
			}
			justBroke = broken
		}

		trickPoints, err := g.rules.CalculateScore(played)
		if err != nil {
			return nil, err
		}

		u.println()
		u.showTrick(g.names, players, played, trickNo, g.handSize)
		u.printf("  %s %sが取りました (%d点)\n",
			strings.TrimSpace(u.arrow()), u.paint(ansiBold, g.names[winner]), trickPoints)

		if err := g.showRoundProgress(taken); err != nil {
			return nil, err
		}
		if justBroke {
			u.printf("  %s\n", u.paint(ansiRed, "※ ハートブレイク！ 以降はハートでリードできます。"))
		}

		leader = winner
		if trickNo < g.handSize {
			g.ui.pause()
		}
	}

	return taken, nil
}

// showRoundProgress はラウンド中の失点経過を表示する。
func (g *Game) showRoundProgress(taken [][]Card) error {
	var parts []string
	for i := range g.names {
		p, err := g.rules.CalculateScore(taken[i])
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("%s %d", g.names[i], p))
	}
	g.ui.printf("  %s\n", g.ui.paint(ansiDim, "経過失点: "+strings.Join(parts, " / ")))
	return nil
}

func (g *Game) askHumanPlay(hand, legal []Card, lead *Card, trickNo int, broken bool) (Card, error) {
	u := g.ui
	shown := u.showHand(hand, legal)

	legalSet := map[Card]bool{}
	for _, c := range legal {
		legalSet[c] = true
	}

	for {
		n, err := u.askInt("出すカードの番号 >", 1, len(shown))
		if err != nil {
			return Card{}, errQuit
		}
		card := shown[n-1]
		if !legalSet[card] {
			reason, err := g.explainIllegal(card, legal, lead, trickNo, broken)
			if err != nil {
				return Card{}, err
			}
			u.printf("  %s は今は出せません: %s\n", u.cardText(card), reason)
			continue
		}
		return card, nil
	}
}

// explainIllegal は出せない理由を日本語で説明する。
// 判定そのものは Prolog が済ませているので、ここは案内文だけを組み立てる。
func (g *Game) explainIllegal(card Card, legal []Card, lead *Card, trickNo int, broken bool) (string, error) {
	// 第1トリックのリードは開始カード固定
	if trickNo == 1 && lead == nil {
		opening, err := g.rules.OpeningCard()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("第1トリックは %s から始めます", g.ui.cardText(opening)), nil
	}

	// マストフォロー
	if lead != nil && card.Suit != lead.Suit {
		for _, c := range legal {
			if c.Suit == lead.Suit {
				return fmt.Sprintf("台札は%sです。%sを持っているので必ず出してください",
					suitName[lead.Suit], suitName[lead.Suit]), nil
			}
		}
	}

	// 第1トリックの点数札
	if trickNo == 1 {
		p, err := g.rules.CardPoint(card)
		if err != nil {
			return "", err
		}
		if p > 0 {
			return "第1トリックでは点数札 (ハート と スペードのQ) を出せません", nil
		}
	}

	// ハートブレイク前のハートリード
	if lead == nil && card.Suit == "h" && !broken {
		return "まだハートが場に出ていないので、ハートからはリードできません", nil
	}

	return "ルール上その手は選べません", nil
}

// openingLeader は第1トリックの開始カード (♣2) を持つプレイヤーを探す。
func (g *Game) openingLeader(hands [][]Card) (int, error) {
	opening, err := g.rules.OpeningCard()
	if err != nil {
		return 0, err
	}
	for i, hand := range hands {
		for _, c := range hand {
			if c == opening {
				g.ui.printf("\n%sが %s を持っているので先手です。\n",
					g.names[i], strings.TrimSpace(g.ui.cardCell(opening)))
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("開始カード %s が誰の手札にもありません", g.ui.cardText(opening))
}

// =============================================
// 小さなヘルパー
// =============================================

func removeCard(hand []Card, card Card) []Card {
	for i, c := range hand {
		if c == card {
			return append(hand[:i:i], hand[i+1:]...)
		}
	}
	return hand
}

func indexOfCard(cards []Card, target Card) int {
	for i, c := range cards {
		if c == target {
			return i
		}
	}
	return 0
}
