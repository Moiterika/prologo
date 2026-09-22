package web

// viewmodel_daifugo.go — 大富豪の Game の状態を画面用の構造体に詰め替える。

import (
	"hearts/internal/cards"
	dgame "hearts/internal/game/daifugo"
	drules "hearts/internal/rules/daifugo"
	"hearts/internal/web/view"
)

// buildDaifugoSetupVM はゲーム開始前のルール選択画面を組み立てる。
func buildDaifugoSetupVM(opts drules.Options, totalGames int) view.DaifugoSetupVM {
	return view.DaifugoSetupVM{
		Groups:     choiceGroups(opts.Groups()),
		TotalGames: totalGames,
	}
}

// choiceGroups は rules 側の選択肢をテンプレート用に写す。
func choiceGroups(groups []drules.Group) []view.GroupVM {
	out := make([]view.GroupVM, 0, len(groups))
	for _, g := range groups {
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
		out = append(out, vm)
	}
	return out
}

func buildDaifugoTableVM(g *dgame.Game, errMsg string) (view.DaifugoTableVM, error) {
	names := g.Names()
	scores := g.Scores()
	titles := g.Titles()
	counts := g.HandCounts()
	out := g.Out()

	seats := make([]view.DaifugoSeatVM, len(names))
	for i, name := range names {
		s := view.DaifugoSeatVM{
			Name:      name,
			IsYou:     i == dgame.HumanSeat,
			Score:     scores[i],
			HandCount: counts[i],
			Turn:      g.Phase() == dgame.PhasePlaying && g.Turn() == i && !out[i],
			Out:       out[i],
			Note:      g.FoulNote(i),
		}
		if i < len(titles) && titles[i] != "" {
			s.Title = dgame.TitleJa(titles[i])
		}
		seats[i] = s
	}

	// 手札。今の場に出せる手に使えないカードは薄くする。
	playable, err := g.PlayableCards()
	if err != nil {
		return view.DaifugoTableVM{}, err
	}
	hand := make([]view.CardVM, 0, len(g.Hand()))
	for i, c := range g.Hand() {
		usable := playable == nil || playable[c]
		hand = append(hand, daifugoCardVM(c, i+1, usable))
	}

	fieldLabel, err := g.FieldLabel()
	if err != nil {
		return view.DaifugoTableVM{}, err
	}
	hasPlay, err := g.HasPlayableHand()
	if err != nil {
		return view.DaifugoTableVM{}, err
	}

	fieldCards := make([]view.CardVM, 0, len(g.LastPlay()))
	for _, c := range g.LastPlay() {
		fieldCards = append(fieldCards, daifugoCardVM(c, 0, false))
	}

	// 今の場に出た手を出た順に。いちばん新しい「出した手」が場を取っている。
	pileEntries := g.Pile()
	lastPlayed := -1
	for i, e := range pileEntries {
		if !e.Passed {
			lastPlayed = i
		}
	}
	pile := make([]view.DaifugoPileVM, 0, len(pileEntries))
	for i, e := range pileEntries {
		vm := view.DaifugoPileVM{
			Passed:  e.Passed,
			Current: i == lastPlayed,
		}
		if e.Player >= 0 && e.Player < len(names) {
			vm.Name = names[e.Player]
			vm.IsYou = e.Player == dgame.HumanSeat
		}
		for _, c := range e.Cards {
			vm.Cards = append(vm.Cards, daifugoCardVM(c, 0, false))
		}
		pile = append(pile, vm)
	}

	received := make([]view.CardVM, 0, len(g.Received()))
	for _, c := range g.Received() {
		received = append(received, daifugoCardVM(c, 0, false))
	}

	events := make([]view.DaifugoEventVM, 0, len(g.Events()))
	for _, e := range g.Events() {
		vm := view.DaifugoEventVM{Text: e.Text}
		if e.Player >= 0 && e.Player < len(names) {
			vm.Name = names[e.Player]
			vm.IsYou = e.Player == dgame.HumanSeat
		}
		for _, c := range e.Cards {
			vm.Cards = append(vm.Cards, daifugoCardVM(c, 0, false))
		}
		events = append(events, vm)
	}

	field := g.Field()
	vm := view.DaifugoTableVM{
		Phase:      string(g.Phase()),
		GameNo:     g.GameNo(),
		TotalGames: g.TotalGames(),
		Seats:      seats,
		Hand:       hand,
		FieldEmpty: field.Empty,
		FieldLabel: fieldLabel,
		FieldCards: fieldCards,
		Pile:       pile,
		Strict:     field.Strict,
		Revolution: g.Revolution(),
		JackBack:   g.JackBack(),
		Reversed:   g.Reversed(),
		OrderLabel: strengthOrder(g.Reversed()),
		OrderNote:  g.ReversalNote(),
		Events:     events,
		YourTurn:   g.YourTurn(),
		CanPass:    g.CanPass(),
		HasPlay:    hasPlay,
		Received:   received,
		RuleLabels: g.Options().Labels(),
		Notice:     g.Notice(),
		Error:      errMsg,
	}
	if len(field.Bound) > 0 {
		vm.BoundLabel = drules.SuitsLabel(field.Bound)
	}
	if lastPlayed >= 0 {
		vm.FieldOwner = pile[lastPlayed].Name
	}
	switch g.Phase() {
	case dgame.PhaseExchange:
		vm.ExchangeNeed = g.ExchangeNeed()
		if to := g.ExchangeTo(); to >= 0 && to < len(names) {
			vm.ExchangeToName = names[to]
		}
	case dgame.PhaseJackBack:
		for _, c := range g.PendingJackBack() {
			vm.JackBackCards = append(vm.JackBackCards, daifugoCardVM(c, 0, false))
		}
	case dgame.PhaseTenDiscard:
		vm.HandOverNeed = g.HandOverNeed()
	case dgame.PhaseSevenGive:
		vm.HandOverNeed = g.HandOverNeed()
		vm.HandOverToName = g.HandOverToName()
	}
	if g.Phase() == dgame.PhaseRoundEnd || g.Phase() == dgame.PhaseGameOver {
		vm.Ranking = daifugoRanking(g)
		vm.LastGame = g.GameNo() >= g.TotalGames()
	}
	if g.Phase() == dgame.PhaseGameOver {
		w := g.Winner()
		vm.WinnerName = names[w]
		vm.YouWin = w == dgame.HumanSeat
	}
	return vm, nil
}

// daifugoRanking は直前のゲームの順位表を組み立てる。
func daifugoRanking(g *dgame.Game) []view.DaifugoRankVM {
	names := g.Names()
	scores := g.Scores()
	titles := g.LastTitles()
	ranking := g.LastRanking()

	out := make([]view.DaifugoRankVM, 0, len(ranking))
	for rank, pi := range ranking {
		if pi < 0 || pi >= len(names) {
			continue
		}
		row := view.DaifugoRankVM{
			Rank:  rank + 1,
			Name:  names[pi],
			IsYou: pi == dgame.HumanSeat,
			Total: scores[pi],
			Note:  g.FoulNote(pi),
		}
		if pi < len(titles) {
			row.Title = dgame.TitleJa(titles[pi])
		}
		if points := g.RankPoints(); rank < len(points) {
			row.Delta = points[rank]
		}
		out = append(out, row)
	}
	return out
}

func daifugoCardVM(c drules.Card, number int, legal bool) view.CardVM {
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

// strengthOrder は今のカードの強さの並びを 1 行で書き表す。
// 革命や Jバックで反転していることに気づかないまま「出せるはずなのに」と
// 悩まないように、盤面に常に出しておく。
func strengthOrder(reversed bool) string {
	if reversed {
		return "2 < A < K < Q < J < 10 < 9 < 8 < 7 < 6 < 5 < 4 < 3 < JOKER"
	}
	return "3 < 4 < 5 < 6 < 7 < 8 < 9 < 10 < J < Q < K < A < 2 < JOKER"
}
