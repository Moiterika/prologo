package web

// viewmodel_napoleon.go — ナポレオンの Game の状態を画面用の構造体に詰め替える。

import (
	"fmt"

	"hearts/internal/cards"
	ngame "hearts/internal/game/napoleon"
	nrules "hearts/internal/rules/napoleon"
	"hearts/internal/web/view"
)

// buildNapoleonSetupVM はゲーム開始前のルール選択画面を組み立てる。
func buildNapoleonSetupVM(opts nrules.Options, totalGames int) view.NapoleonSetupVM {
	groups := make([]view.GroupVM, 0)
	for _, g := range opts.Groups() {
		vm := view.GroupVM{
			Name:          g.Name,
			Legend:        g.Legend,
			TogglesNote:   g.TogglesNote,
			TogglesHideOn: g.TogglesHideOn,
			TogglesHidden: g.TogglesHidden(),
		}
		for _, c := range g.Choices {
			vm.Choices = append(vm.Choices, view.ChoiceVM{
				Value:       c.Value,
				Label:       c.Label,
				Description: c.Description,
				Checked:     c.Value == g.Selected,
			})
		}
		for _, t := range g.Toggles {
			vm.Toggles = append(vm.Toggles, view.ToggleVM{
				Name:        t.Name,
				Label:       t.Label,
				Description: t.Description,
				Checked:     t.On,
			})
		}
		groups = append(groups, vm)
	}
	return view.NapoleonSetupVM{Groups: groups, TotalGames: totalGames}
}

func buildNapoleonTableVM(g *ngame.Game, errMsg string) (view.NapoleonTableVM, error) {
	names := g.Names()
	scores := g.Scores()
	areas := seatAreas(len(names))
	deltas := g.RoundDeltas()

	// このトリックで誰が何番目に何を出したか
	type play struct {
		card  nrules.Card
		order int
	}
	plays := map[int]play{}
	for i, pi := range g.PlayedBy() {
		plays[pi] = play{card: g.Played()[i], order: i + 1}
	}

	// 取った絵札の枚数。トリックが片付くたびに増えるので、そのまま盤面に出す。
	taken, err := g.TakenPoints()
	if err != nil {
		return view.NapoleonTableVM{}, err
	}

	seats := make([]view.NapoleonSeatVM, len(names))
	for i, name := range names {
		s := view.NapoleonSeatVM{
			Name:  name,
			IsYou: i == ngame.HumanSeat,
			Score: scores[i],
			Role:  napoleonRole(g, i),
			Turn:  g.Phase() == ngame.PhasePlaying && g.Turn() == i,
		}
		if i < len(taken) {
			s.TakenPoints = taken[i]
		}
		if i < len(areas) {
			s.Area = areas[i]
		}
		if i < len(deltas) {
			s.Delta = deltas[i]
		}
		if p, ok := plays[i]; ok {
			card := napoleonCardVM(p.card, 0, false)
			s.Played = &card
			s.PlayOrder = p.order
			s.Winner = g.Phase() == ngame.PhaseTrickDone && i == g.TrickWinner()
		}
		seats[i] = s
	}

	legal, err := g.LegalCards()
	if err != nil {
		return view.NapoleonTableVM{}, err
	}
	legalSet := make(map[nrules.Card]bool, len(legal))
	for _, c := range legal {
		legalSet[c] = true
	}
	hand := make([]view.CardVM, 0, len(g.Hand()))
	for i, c := range g.Hand() {
		hand = append(hand, napoleonCardVM(c, i+1, legalSet[c]))
	}

	got, err := g.NapoleonPoints()
	if err != nil {
		return view.NapoleonTableVM{}, err
	}

	vm := view.NapoleonTableVM{
		Phase:       string(g.Phase()),
		GameNo:      g.GameNo(),
		TotalGames:  g.TotalGames(),
		Seats:       seats,
		Hand:        hand,
		BidHistory:  g.BidHistory(),
		WidowSize:   g.WidowSize(),
		MaxPoints:   g.MaxPoints(),
		GotPoints:   got,
		TrickNo:     g.TrickNo(),
		TrickTotal:  g.TrickTotal(),
		TrickPoints: g.TrickPoints(),
		CenterText:  napoleonCenterText(g),
		Solo:        g.Solo(),
		RuleLabels:  g.Options().Labels(),
		Notice:      g.Notice(),
		Error:       errMsg,
		LastGame:    g.GameNo() >= g.TotalGames(),
	}

	if bid := g.Bid(); bid != nil {
		vm.CurrentBid = bid.Label()
		vm.BidCount = bid.Count
	}
	if trump := g.Trump(); trump != "" {
		vm.TrumpSymbol = cards.SuitSymbol(trump)
		vm.TrumpName = cards.SuitName(trump)
	}
	if suit := g.LeadSuit(); suit != "" && suit != nrules.NoSuit {
		vm.LeadSuit = cards.SuitSymbol(suit)
	}
	if n := g.NapoleonSeat(); n >= 0 {
		vm.NapoleonName = names[n]
	}
	if a := g.AideSeat(); a >= 0 && g.AideRevealed() {
		vm.AideName = names[a]
	}
	// 指名したカードは公開情報 (誰が持っているかだけが分からない) なので、
	// 決まった時点からずっと出しておく。
	if card := g.AideCard(); card.Suit != "" {
		vm.AideCardLabel = cards.Label(card)
		vm.AideCardRed = cards.IsRedSuit(card.Suit)
	}
	if label := ngame.ReasonJa(g.TrickReason()); label != "" && g.Phase() == ngame.PhaseTrickDone {
		vm.TrickReason = label
	}

	switch g.Phase() {
	case ngame.PhaseBidding:
		vm.YourBidTurn = g.BidTurn() == ngame.HumanSeat
		if vm.YourBidTurn {
			bids, err := g.AvailableBids()
			if err != nil {
				return view.NapoleonTableVM{}, err
			}
			vm.AvailableBids = napoleonBidVMs(bids)
		}
		vm.MinBid = g.Options().MinBid
	case ngame.PhaseNaming:
		for _, c := range g.AideChoices() {
			vm.AideChoices = append(vm.AideChoices, view.AideChoiceVM{
				Suit:  c.Suit,
				Rank:  c.Rank,
				Label: cards.Label(c),
				Red:   cards.IsRedSuit(c.Suit),
			})
		}
	case ngame.PhaseRoundDone:
		r := g.Result()
		vm.NapoleonWin = r.Outcome == "napoleon"
		if vm.NapoleonWin {
			vm.OutcomeLabel = fmt.Sprintf("ナポレオン軍の勝ち！ 宣言 %d 枚に対して %d 枚獲得。",
				vm.BidCount, g.RoundTaken())
		} else {
			vm.OutcomeLabel = fmt.Sprintf("連合軍の勝ち！ 宣言 %d 枚に対して %d 枚しか取れませんでした。",
				vm.BidCount, g.RoundTaken())
		}
	case ngame.PhaseGameOver:
		w := g.Winner()
		vm.WinnerName = names[w]
		vm.YouWin = w == ngame.HumanSeat
	}
	return vm, nil
}

// napoleonRole は席の立場。副官は正体が割れるまで伏せる。
func napoleonRole(g *ngame.Game, pi int) string {
	if g.NapoleonSeat() < 0 {
		return ""
	}
	if pi == g.NapoleonSeat() {
		return "ナポレオン"
	}
	if g.Solo() {
		return "連合軍"
	}
	if !g.AideRevealed() {
		return "？"
	}
	if pi == g.AideSeat() {
		return "副官"
	}
	return "連合軍"
}

func napoleonCenterText(g *ngame.Game) string {
	names := g.Names()
	switch g.Phase() {
	case ngame.PhaseTrickDone:
		return fmt.Sprintf("%sが取りました (絵札 %d 枚)", names[g.TrickWinner()], g.TrickPoints())
	case ngame.PhasePlaying:
		if g.YourTurn() {
			return "あなたの番です"
		}
		return names[g.Turn()] + " の番です"
	}
	return ""
}

func napoleonBidVMs(bids []nrules.Bid) []view.BidVM {
	// 選択肢が多すぎると押しにくいので、弱い方から少しだけ出す。
	const maxShown = 12
	if len(bids) > maxShown {
		bids = bids[:maxShown]
	}
	out := make([]view.BidVM, 0, len(bids))
	for _, b := range bids {
		out = append(out, view.BidVM{
			Count: b.Count,
			Suit:  b.Suit,
			Label: b.Label(),
			Red:   cards.IsRedSuit(b.Suit),
		})
	}
	return out
}

func napoleonCardVM(c nrules.Card, number int, legal bool) view.CardVM {
	if c.IsJoker() {
		return view.CardVM{Number: number, Suit: "★", Rank: "JOKER", Red: true, Legal: legal}
	}
	return view.CardVM{
		Number: number,
		Suit:   cards.SuitSymbol(c.Suit),
		Rank:   cards.RankLabel(c.Rank),
		Red:    cards.IsRedSuit(c.Suit),
		Legal:  legal,
	}
}
