package main

// options.go — ゲーム開始時に選ぶ流儀。
//
// 実際の判定は Prolog 側 (hearts_rules.pl の first_trick_points/1,
// first_trick_lead/1) が持つ。ここはその設定と、画面に出す説明文だけ。

// Options は選べる流儀。
type Options struct {
	FirstTrickPoints FirstTrickPoints
	FirstTrickLead   FirstTrickLead
	HeartsBreak      HeartsBreakRule
}

// DefaultOptions は既定の流儀。
// 第1トリックの点数札は許可、ハートブレイク規則は使わない。
func DefaultOptions() Options {
	return Options{
		FirstTrickPoints: AllowPoints,
		FirstTrickLead:   LeadOpeningCard,
		HeartsBreak:      HeartsBreakOff,
	}
}

// Apply は Prolog 側の設定を書き換える。
func (o Options) Apply(r *Rules) error {
	if err := r.SetFirstTrickPoints(o.FirstTrickPoints); err != nil {
		return err
	}
	if err := r.SetFirstTrickLead(o.FirstTrickLead); err != nil {
		return err
	}
	return r.SetHeartsBreakRule(o.HeartsBreak)
}

// UsesHeartsBreak はハートブレイク規則を使う設定か。
func (o Options) UsesHeartsBreak() bool {
	return o.HeartsBreak == HeartsBreakOn
}

// ParseOptions はフォームの値を Options にする。
// 知らない値は既定に落とす (画面から来る値なので厳密に弾く必要はない)。
func ParseOptions(points, lead, heartsBreak string) Options {
	o := DefaultOptions()
	switch FirstTrickPoints(points) {
	case ForbidAllPoints, ForbidHearts, AllowPoints:
		o.FirstTrickPoints = FirstTrickPoints(points)
	}
	switch FirstTrickLead(lead) {
	case LeadOpeningCard, LeadFree:
		o.FirstTrickLead = FirstTrickLead(lead)
	}
	switch HeartsBreakRule(heartsBreak) {
	case HeartsBreakOn, HeartsBreakOff:
		o.HeartsBreak = HeartsBreakRule(heartsBreak)
	}
	return o
}

// PointsLabel は選んだ流儀の短い説明。
func (o Options) PointsLabel() string {
	switch o.FirstTrickPoints {
	case ForbidHearts:
		return "第1トリックは ♥ 禁止 (♠Q は可)"
	case AllowPoints:
		return "第1トリックの点数札制限なし"
	default:
		return "第1トリックは点数札禁止"
	}
}

// LeadLabel は選んだ流儀の短い説明。
func (o Options) LeadLabel() string {
	if o.FirstTrickLead == LeadFree {
		return "♣2 の持ち主が自由にリード"
	}
	return "♣2 スタート"
}

// HeartsBreakLabel は選んだ流儀の短い説明。
func (o Options) HeartsBreakLabel() string {
	if o.UsesHeartsBreak() {
		return "ハートブレイクあり"
	}
	return "ハートブレイクなし"
}
