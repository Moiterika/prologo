package web

// viewmodel.go — Game の状態を画面用の構造体に詰め替える。
// テンプレート側で判断が要らないよう、ここで表示に必要な形まで整える。

import (
	"fmt"

	"hearts/internal/game"
	"hearts/internal/rules"
	"hearts/internal/web/view"
)

// passDirJa は Prolog が返すパス方向 (pass_direction/2) の日本語表記。
var passDirJa = map[string]string{
	"left":   "左隣",
	"right":  "右隣",
	"across": "向かい",
	"hold":   "交換なし",
}

// buildSetupVM はゲーム開始前のルール選択画面を組み立てる。
func buildSetupVM(engine *rules.Engine, current rules.Options, limit int) (view.SetupVM, error) {
	maxPoints, err := engine.TotalRoundPoints()
	if err != nil {
		return view.SetupVM{}, err
	}
	opening, err := engine.OpeningCard()
	if err != nil {
		return view.SetupVM{}, err
	}
	open := rules.CardLabel(opening)

	// 制限のゆるい順に並べる。
	points := []view.OptionVM{
		{
			Value:       string(rules.AllowPoints),
			Label:       "点数札許可 (♥ 全般・♠Q 許可)",
			Description: "1巡目の制限なし。台札のスートが無ければハートも ♠Q も自由に捨てられます。",
		},
		{
			Value:       string(rules.ForbidHearts),
			Label:       "♥ のみ禁止 (♠Q 許可)",
			Description: "1巡目にハートは出せませんが、台札のスートが無ければ ♠Q は捨てられます。",
		},
		{
			Value:       string(rules.ForbidAllPoints),
			Label:       "点数札禁止 (♥ 全般・♠Q は禁止)",
			Description: "no blood on the first trick。1巡目に失点が動きません。",
		},
	}
	lead := []view.OptionVM{
		{
			Value:       string(rules.LeadOpeningCard),
			Label:       "第1トリックは " + open + " から始める",
			Description: open + " を持っている人が、必ずそのカードでリードします。",
		},
		{
			Value:       string(rules.LeadFree),
			Label:       "第1トリックは " + open + " を持つ人が自由に出せる",
			Description: open + " を持っている人が先手になりますが、出すカードは自由に選べます。",
		},
	}

	heartsBreak := []view.OptionVM{
		{
			Value:       string(rules.HeartsBreakOff),
			Label:       "なし",
			Description: "いつでもハートでリードできます。",
		},
		{
			Value:       string(rules.HeartsBreakOn),
			Label:       "あり",
			Description: "得点札 (♥ 全般と ♠Q) が場に出るまで、ハートからリードできません。手札がハートだけの場合は例外です。",
		},
	}

	for i := range points {
		points[i].Checked = points[i].Value == string(current.FirstTrickPoints)
	}
	for i := range lead {
		lead[i].Checked = lead[i].Value == string(current.FirstTrickLead)
	}
	for i := range heartsBreak {
		heartsBreak[i].Checked = heartsBreak[i].Value == string(current.HeartsBreak)
	}

	return view.SetupVM{
		PointsOptions:      points,
		LeadOptions:        lead,
		HeartsBreakOptions: heartsBreak,
		MaxPoints:          maxPoints,
		Limit:              limit,
	}, nil
}

// seatAreas は円卓のどこに誰を置くかを決める。
// 「あなた」(0 番) を必ず手前 (bottom) にして、そこから席順に左回りで並べる。
func seatAreas(players int) []string {
	switch players {
	case 3:
		return []string{"bottom", "left", "right"}
	case 5:
		return []string{"bottom", "left", "topleft", "topright", "right"}
	default:
		return []string{"bottom", "left", "top", "right"}
	}
}

func buildTableVM(g *game.Game, errMsg string) (view.TableVM, error) {
	takenPoints, err := g.TakenPoints()
	if err != nil {
		return view.TableVM{}, err
	}
	takenCounts := g.TakenCounts()

	names := g.Names()
	scores := g.Scores()
	final := g.RoundFinal()
	areas := seatAreas(len(names))

	// このトリックで誰が何番目に何を出したか
	type play struct {
		card  rules.Card
		order int
	}
	plays := map[int]play{}
	for i, pi := range g.PlayedBy() {
		plays[pi] = play{card: g.Played()[i], order: i + 1}
	}

	seats := make([]view.SeatVM, len(names))
	for i, name := range names {
		s := view.SeatVM{
			Name:       name,
			IsYou:      i == game.HumanSeat,
			Total:      scores[i],
			RoundTaken: takenPoints[i],
			TakenCount: takenCounts[i],
			Turn:       g.Phase() == game.PhasePlaying && g.Turn() == i,
		}
		if i < len(areas) {
			s.Area = areas[i]
		}
		if i < len(final) {
			s.RoundDelta = final[i]
		}
		if p, ok := plays[i]; ok {
			card := cardVM(p.card, 0, false)
			s.Played = &card
			s.PlayOrder = p.order
			// 勝者のカードはトリックが揃ってから光らせる
			s.Winner = g.Phase() == game.PhaseTrickDone && i == g.TrickWinner()
		}
		seats[i] = s
	}

	// 手札。出せるカードに印を付ける。
	legal, err := g.LegalCards()
	if err != nil {
		return view.TableVM{}, err
	}
	legalSet := make(map[rules.Card]bool, len(legal))
	for _, c := range legal {
		legalSet[c] = true
	}
	hand := make([]view.CardVM, 0, len(g.Hand()))
	for i, c := range g.Hand() {
		hand = append(hand, cardVM(c, i+1, legalSet[c]))
	}

	received := make([]view.CardVM, 0, len(g.Received()))
	for _, c := range g.Received() {
		received = append(received, cardVM(c, 0, false))
	}

	vm := view.TableVM{
		Phase:           string(g.Phase()),
		Round:           g.Round(),
		Limit:           g.Limit(),
		MaxPoints:       g.MaxPoints(),
		TrickNo:         g.TrickNo(),
		TrickTotal:      g.TrickTotal(),
		Seats:           seats,
		Hand:            hand,
		CenterText:      centerText(g),
		PassCount:       g.PassCount(),
		PassDir:         passDirJa[g.PassDir()],
		Received:        received,
		TrickPoints:     g.TrickPoints(),
		HeartsBroken:    g.HeartsBroken(),
		YourTurn:        g.YourTurn(),
		Notice:          g.Notice(),
		Error:           errMsg,
		ShowHeartsBreak: g.Options().UsesHeartsBreak(),
		RuleLabels: []string{
			g.Options().LeadLabel(),
			g.Options().PointsLabel(),
			g.Options().HeartsBreakLabel(),
		},
	}

	if lead := g.Lead(); lead != nil {
		vm.LeadSuit = rules.SuitSymbol(lead.Suit)
	}
	if s := g.RoundSlammer(); s >= 0 {
		vm.SlammerName = names[s]
	}
	if g.Phase() == game.PhaseGameOver {
		w := g.Winner()
		vm.WinnerName = names[w]
		vm.YouWin = w == game.HumanSeat
	}
	return vm, nil
}

// centerText は円卓の中央に出す一言。
func centerText(g *game.Game) string {
	names := g.Names()
	switch g.Phase() {
	case game.PhaseTrickDone:
		return fmt.Sprintf("%sが取りました (%d 点)", names[g.TrickWinner()], g.TrickPoints())
	case game.PhasePlaying:
		if g.YourTurn() {
			return "あなたの番です"
		}
		return names[g.Turn()] + " の番です"
	}
	return ""
}

func cardVM(c rules.Card, number int, legal bool) view.CardVM {
	return view.CardVM{
		Number: number,
		Suit:   rules.SuitSymbol(c.Suit),
		Rank:   rules.RankLabel(c.Rank),
		Red:    rules.IsRedSuit(c.Suit),
		Legal:  legal,
	}
}
