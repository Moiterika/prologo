package view

// types_daifugo.go — 大富豪の画面に渡すビューモデル。
//
// types.go (ハート) と同じ方針で、テンプレート側で計算や判断をしないで済むよう、
// 表示に必要な形まで完全に整えたものだけを持つ。

// ChoiceVM は設定画面のラジオボタン 1 個。
type ChoiceVM struct {
	Value       string
	Label       string
	Description string
	Checked     bool
}

// ToggleVM はラジオボタンに添える on/off のチェックボックス 1 個。
// 「縛り」に対する激縛り・片方縛りのように、選んだ流儀をさらに細かくする。
type ToggleVM struct {
	Name        string
	Label       string
	Description string
	Checked     bool
}

// GroupVM は設定画面の 1 つの問い。
type GroupVM struct {
	Name        string
	Legend      string
	Choices     []ChoiceVM
	Toggles     []ToggleVM
	TogglesNote string
	// TogglesHideOn は「この値を選んでいるあいだ Toggles を出さない」選択肢。
	// 空なら常に出す。
	TogglesHideOn string
	// TogglesHidden は最初の描画で Toggles を隠すか。
	TogglesHidden bool
}

// PresetVM はルール選択画面のプリセット 1 つ。
// 押すと、ローカルルールの選択がまとめてその組み合わせに置き換わる。
type PresetVM struct {
	Name        string
	Label       string
	Description string
	Active      bool // 今の選択がこのプリセットと一致しているか
}

// DaifugoSetupVM は大富豪のルール選択画面。
type DaifugoSetupVM struct {
	Presets    []PresetVM
	Groups     []GroupVM
	TotalGames int
	Notice     string // プリセットを反映したときの案内
}

// DaifugoSeatVM は大富豪の 1 プレイヤー。
type DaifugoSeatVM struct {
	Name      string
	IsYou     bool
	Title     string // 前のゲームの称号。初回は空
	Score     int    // 累計の順位点
	HandCount int    // 残り手札枚数
	Turn      bool   // 今この人の手番か
	Out       bool   // 上がり済み (または都落ち)
	Note      string // 「反則上がり」「都落ち」
}

// DaifugoRankVM はゲーム結果の 1 行。
type DaifugoRankVM struct {
	Rank  int
	Name  string
	IsYou bool
	Title string
	Delta int
	Total int
	Note  string
}

// DaifugoEventVM は場の履歴 1 件。
type DaifugoEventVM struct {
	Name  string // 場全体の出来事なら空
	IsYou bool
	Text  string
	Cards []CardVM
}

// DaifugoPileVM は「今の場」に積まれた 1 手。誰が何を出したかを見せる。
type DaifugoPileVM struct {
	Name    string
	IsYou   bool
	Cards   []CardVM
	Passed  bool
	Current bool // 今この手が場を取っているか (いちばん新しい手)
}

// DaifugoTableVM は大富豪の盤面 1 枚分。
type DaifugoTableVM struct {
	// 局面: exchange / playing / round_end / game_over
	Phase string

	GameNo     int
	TotalGames int

	Seats []DaifugoSeatVM
	Hand  []CardVM

	// 場
	FieldEmpty bool
	FieldLabel string
	FieldCards []CardVM
	FieldOwner string          // 今の場を取っている人。場が空なら空
	Pile       []DaifugoPileVM // 今の場に出た手を出た順に
	BoundLabel string          // 縛られているマーク。無ければ空
	Strict     bool            // 激縛り中か

	Revolution bool   // 革命中
	JackBack   bool   // Jバック中
	Reversed   bool   // 実際に強弱が反転しているか
	OrderLabel string // 今の強さの並び (「3 < 4 < … < 2 < JOKER」)
	OrderNote  string // 革命・Jバックについての一言。無ければ空

	Events []DaifugoEventVM

	YourTurn bool
	CanPass  bool
	HasPlay  bool // 出せる手があるか

	// カード交換 (exchange)
	ExchangeNeed   int
	ExchangeToName string
	Received       []CardVM

	// Jバックの可否 (jack_back)
	JackBackCards []CardVM

	// 10捨て・7渡し (ten_discard / seven_give)
	HandOverNeed   int
	HandOverToName string

	// ゲーム結果 (round_end)
	Ranking  []DaifugoRankVM
	LastGame bool // これが最後のゲームか

	// 全ゲーム終了 (game_over)
	WinnerName string
	YouWin     bool

	RuleLabels []string
	Notice     string
	Error      string
}
