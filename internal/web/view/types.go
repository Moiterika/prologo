package view

import "strconv"

// types.go — テンプレートに渡すビューモデル。
//
// テンプレート側で計算や判断をしないで済むよう、表示に必要な形まで
// 完全に整えたものだけを持つ。

// itoa はテンプレートから数値を書き出すための小さな補助。
func itoa(n int) string { return strconv.Itoa(n) }

// totalLabel は得点表の見出し。体力制では持ち点、それ以外は累計失点。
func totalLabel(life bool) string {
	if life {
		return "持ち点"
	}
	return "累計"
}

// OptionVM は設定画面のラジオボタン 1 個。
type OptionVM struct {
	Value       string
	Label       string
	Description string
	Checked     bool
}

// SetupVM はゲーム開始前のルール選択画面。
type SetupVM struct {
	Notice             string // プリセットを反映したときの案内
	PointsOptions      []OptionVM
	LeadOptions        []OptionVM
	HeartsBreakOptions []OptionVM
	ScoringOptions     []OptionVM
	MaxPoints          int
	Limit              int
}

// CardVM は 1 枚のカード。
type CardVM struct {
	Number int    // 手札の表示番号 (1 始まり)。場のカードでは 0。
	Suit   string // ♠ ♥ ♦ ♣
	Rank   string // A, K, Q, J, 10, 9 ...
	Red    bool   // ♥♦ なら真
	Legal  bool   // 今出せるカードか
}

// SeatVM は円卓の 1 席。得点表の 1 行としても使う。
type SeatVM struct {
	Name       string
	IsYou      bool
	Total      int  // 累計失点。体力制では持ち点
	RoundTaken int  // このラウンドで取った失点
	TakenCount int  // このラウンドで取った枚数
	RoundDelta int  // このラウンドの点数の増減 (ラウンド結果でのみ使う)。体力制では持ち点の増減
	Turn       bool // 今この人の手番か

	// 円卓のどこに置くか: bottom / left / top / right / topleft / topright
	Area string

	Played    *CardVM // このトリックで出したカード。まだなら nil
	PlayOrder int     // このトリックで何番目に出したか (1 始まり)
	Winner    bool    // このトリックを取ったか
}

// TableVM は画面 1 枚分。
type TableVM struct {
	// 局面: passing / playing / trick_done / round_done / game_over
	Phase string

	Round      int
	Limit      int
	MaxPoints  int
	TrickNo    int
	TrickTotal int

	Seats []SeatVM // プレイヤー番号順 (0 = あなた)
	Hand  []CardVM

	// 円卓の中央に出す一言 (「あなたの番」「CPU2 が取りました」など)
	CenterText string

	// 交換 (passing)
	PassCount int
	PassDir   string   // 「左隣」など
	Received  []CardVM // 直前に受け取ったカード

	// トリック終了 (trick_done)
	TrickPoints int

	// ラウンド終了 (round_done)
	SlammerName string // スラム達成者。いなければ空
	SlamText    string // スラムの精算の説明

	// ゲーム終了 (game_over)
	WinnerName string
	YouWin     bool

	// 共通
	RuleLabels []string // 開始時に選んだ流儀の説明
	LeadSuit   string   // 台札のスート記号。リード前は空

	// ハートブレイク規則を使う設定のときだけ、その状態を表示する
	ShowHeartsBreak bool

	// 体力制 (持ち点から減らしていく) か
	Life bool
	HeartsBroken    bool

	YourTurn bool
	Notice   string // ハートブレイクなどの案内
	Error    string // 入力ミスの指摘
}

// seatArea は席を 3x3 のどのマスに置くかを Tailwind のクラスで返す。
func seatArea(area string) string {
	switch area {
	case "bottom":
		return "col-start-2 row-start-3"
	case "left":
		return "col-start-1 row-start-2"
	case "top":
		return "col-start-2 row-start-1"
	case "right":
		return "col-start-3 row-start-2"
	case "topleft":
		return "col-start-1 row-start-1"
	case "topright":
		return "col-start-3 row-start-1"
	}
	return ""
}

// seatFlow は名札と出したカードの並び方を返す。
// 出したカードが常に円卓の中央側に来るようにする。
func seatFlow(area string) string {
	switch area {
	case "bottom":
		return "flex-col-reverse"
	case "left":
		return "flex-row"
	case "right":
		return "flex-row-reverse"
	default: // top / topleft / topright
		return "flex-col"
	}
}

// signed は符号つきで数値を書き出す。ナポレオンの得点は正負どちらもある。
func signed(n int) string {
	if n > 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// HomeVM はゲームを選ぶ画面。対局の途中のゲームには印を付ける。
type HomeVM struct {
	HeartsPlaying   bool
	DaifugoPlaying  bool
	NapoleonPlaying bool
}
