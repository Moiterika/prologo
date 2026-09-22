package view

// types_napoleon.go — ナポレオンの画面に渡すビューモデル。

// NapoleonSetupVM はナポレオンのルール選択画面。
type NapoleonSetupVM struct {
	Groups     []GroupVM
	TotalGames int
}

// BidVM は選べるナポレオン宣言 1 つ。
type BidVM struct {
	Count int
	Suit  string // Prolog 側のアトム (s/h/d/c)。フォームの値に使う
	Label string // "♠ で 13 枚"
	Red   bool
}

// AideChoiceVM は副官として指名できるカード 1 枚。
type AideChoiceVM struct {
	Suit  string // フォームの値
	Rank  string // フォームの値
	Label string // "♠A"
	Red   bool
}

// NapoleonSeatVM はナポレオンの 1 席。
type NapoleonSeatVM struct {
	Name        string
	IsYou       bool
	Score       int
	Role        string // 「ナポレオン」「副官」「連合軍」。未判明なら空
	Turn        bool
	Delta       int // このゲームの増減 (精算後に使う)
	TakenPoints int // このゲームでこれまでに取った絵札の枚数

	// 円卓のどこに置くか
	Area string

	Played    *CardVM
	PlayOrder int
	Winner    bool
}

// NapoleonTableVM はナポレオンの盤面 1 枚分。
type NapoleonTableVM struct {
	// 局面: bidding / naming / discarding / playing / trick_done /
	//       round_done / game_over
	Phase string

	GameNo     int
	TotalGames int

	Seats []NapoleonSeatVM
	Hand  []CardVM

	// 競り
	BidHistory    []string
	CurrentBid    string // 今の宣言。まだ無ければ空
	AvailableBids []BidVM
	MinBid        int
	YourBidTurn   bool

	// 副官の指名
	AideChoices []AideChoiceVM

	// 埋め札
	WidowSize int

	// 対戦の情報
	TrumpSymbol   string
	TrumpName     string
	BidCount      int
	NapoleonName  string
	AideName      string // 未判明なら空
	AideCardLabel string // 指名が決まったあとはずっと入れる (公開情報)
	AideCardRed   bool   // 指名したカードが ♥♦ か
	Solo          bool
	MaxPoints     int
	GotPoints     int // ナポレオン軍がこれまでに取った絵札

	TrickNo    int
	TrickTotal int
	LeadSuit   string // 台札のマーク記号。定まらないときは空

	CenterText  string
	TrickPoints int
	TrickReason string // 「セイムツー」など。通常の勝ちなら空

	// 精算 (round_done)
	OutcomeLabel string
	NapoleonWin  bool
	LastGame     bool

	// 全ゲーム終了 (game_over)
	WinnerName string
	YouWin     bool

	RuleLabels []string
	Notice     string
	Error      string
}
