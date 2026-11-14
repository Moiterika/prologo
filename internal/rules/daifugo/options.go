package daifugo

// options.go — ゲーム開始前に選ぶローカルルール。
//
// 判定そのものは Prolog 側 (daifugo_rules.pl の動的述語) が持つ。
// ここにあるのは「どの値を選べるか」と「画面に出す説明文」だけ。

import "fmt"

// Options は選べるローカルルール一式。値は全て Prolog 側のアトムと同じ文字列。
type Options struct {
	Players       int    // 人数 3〜6
	Jokers        int    // ジョーカーの枚数 0/1/2
	EightCut      string // off / all / group_only
	JackBack      string // off / all / group_only
	JackBackMode  string // forced / optional
	SeqRevolution string // off / on
	FiveCard      string // off / on
	Bind          string // off / on
	BindStrict    string // off / on  … 激縛り (縛りの一種)
	BindPartial   string // off / on  … 片方縛り (縛りの一種)
	TenDiscard    string // off / on
	SevenGive     string // off / on
	SpadeThree    string // off / on
	PassLock      string // off / on
	FinishFlush   string // off / on
	FoulFinish    string // off / on
	MiyakoOchi    string // off / on
	FirstPlay     string // diamond3_must / diamond3_free / free
}

// DefaultOptions は既定のローカルルール。
// ローカルルールを盛った、賑やかな卓に寄せてある。
// 7渡しとパス禁止・上がり即流しだけは入れていない。
func DefaultOptions() Options {
	return Options{
		Players:       4,
		Jokers:        2,
		EightCut:      "all",
		JackBack:      "all",
		JackBackMode:  "optional",
		SeqRevolution: "on",
		FiveCard:      "on",
		Bind:          "on",
		BindStrict:    "on",
		BindPartial:   "on",
		TenDiscard:    "on",
		SevenGive:     "off",
		SpadeThree:    "on",
		PassLock:      "off",
		FinishFlush:   "off",
		FoulFinish:    "on",
		MiyakoOchi:    "on",
		FirstPlay:     "diamond3_free",
	}
}

// Apply は Prolog 側の設定を書き換える。
// 不正な値は Prolog の member/2 が弾くので、ここでは検証しない。
func (o Options) Apply(e *Engine) error {
	steps := []struct {
		query string
		what  string
	}{
		{fmt.Sprintf("set_players(%d).", o.Players), "人数"},
		{fmt.Sprintf("set_joker_count(%d).", o.Jokers), "ジョーカーの枚数"},
		{fmt.Sprintf("set_eight_cut(%s).", o.EightCut), "8流し"},
		{fmt.Sprintf("set_jack_back(%s).", o.JackBack), "Jバック"},
		{fmt.Sprintf("set_jack_back_mode(%s).", o.JackBackMode), "Jバックの適用"},
		{fmt.Sprintf("set_seq_revolution(%s).", o.SeqRevolution), "階段革命"},
		{fmt.Sprintf("set_five_card(%s).", o.FiveCard), "ファイブカード"},
		{fmt.Sprintf("set_bind_rule(%s).", o.Bind), "縛り"},
		{fmt.Sprintf("set_bind_strict(%s).", o.BindStrict), "激縛り"},
		{fmt.Sprintf("set_bind_partial(%s).", o.BindPartial), "片方縛り"},
		{fmt.Sprintf("set_ten_discard(%s).", o.TenDiscard), "10捨て"},
		{fmt.Sprintf("set_seven_give(%s).", o.SevenGive), "7渡し"},
		{fmt.Sprintf("set_spade_three(%s).", o.SpadeThree), "スペ3返し"},
		{fmt.Sprintf("set_pass_lock(%s).", o.PassLock), "パスの処理"},
		{fmt.Sprintf("set_finish_flush(%s).", o.FinishFlush), "上がり時の処理"},
		{fmt.Sprintf("set_foul_finish_rule(%s).", o.FoulFinish), "反則上がり"},
		{fmt.Sprintf("set_miyako_ochi(%s).", o.MiyakoOchi), "都落ち"},
		{fmt.Sprintf("set_first_play_rule(%s).", o.FirstPlay), "最初の一手"},
	}
	for _, s := range steps {
		if err := e.p.Set(s.query, s.what); err != nil {
			return err
		}
	}
	return nil
}

// UsesPassLock はパスしたら場が流れるまで出せない設定か。
func (o Options) UsesPassLock() bool { return o.PassLock == "on" }

// UsesFinishFlush は誰かが上がった瞬間に場を流す設定か。
func (o Options) UsesFinishFlush() bool { return o.FinishFlush == "on" }

// UsesMiyakoOchi は都落ちを使う設定か。
func (o Options) UsesMiyakoOchi() bool { return o.MiyakoOchi == "on" }

// =============================================
// 画面に出す選択肢
// =============================================

// Choice は設定画面のラジオボタン 1 個ぶん。
type Choice struct {
	Value       string
	Label       string
	Description string
}

// Toggle はラジオボタンに添える on/off のチェックボックス 1 個ぶん。
// 「縛り」に対する激縛り・片方縛りのように、選んだ流儀をさらに細かくする
// 枝分かれを表す。
type Toggle struct {
	Name        string // フォームの name 属性
	Label       string
	Description string
	On          bool
}

// Group は設定画面の 1 つの問い。
//
// 選択肢は「なし」を先頭に置く。使わない流儀が上、盛るほど下、という並びで
// 揃えておくと、どこまで盛ったかが一目で分かる。
type Group struct {
	Name     string // フォームの name 属性
	Legend   string // 見出し
	Choices  []Choice
	Selected string
	Toggles  []Toggle // 追加の on/off。無ければ空
	// TogglesNote は Toggles の上に出す注釈 (「縛りを使うときだけ効きます」など)。
	TogglesNote string
	// TogglesHideOn は「この値を選んでいるあいだ Toggles を出さない」選択肢。
	// 親の流儀を使わないのに枝分かれだけ見えていても仕方がないため。
	TogglesHideOn string
}

// TogglesHidden は今の選択で Toggles を隠すべきか。
func (g Group) TogglesHidden() bool {
	return g.TogglesHideOn != "" && g.Selected == g.TogglesHideOn
}

// Groups は設定画面に並べる問いを、今の選択を反映して返す。
func (o Options) Groups() []Group {
	onOff := func(v string) bool { return v == "on" }
	return []Group{
		{
			Name:     "players",
			Legend:   "人数",
			Selected: fmt.Sprint(o.Players),
			Choices: []Choice{
				{"3", "3人", "1 人あたり 18 枚前後。長丁場になります。"},
				{"4", "4人", "1 人あたり 13 枚前後。もっとも遊びやすい人数です。"},
				{"5", "5人", "1 人あたり 11 枚前後。"},
				{"6", "6人", "1 人あたり 9 枚前後。1 ゲームが短く終わります。"},
			},
		},
		{
			Name:     "jokers",
			Legend:   "ジョーカーの枚数",
			Selected: fmt.Sprint(o.Jokers),
			Choices: []Choice{
				{"0", "なし (52枚)", "ジョーカーを抜いた 52 枚で遊びます。"},
				{"1", "1枚 (53枚)", "ジョーカーを 1 枚だけ入れます。"},
				{"2", "2枚 (54枚)", "基本ルール。ジョーカーは何の代わりにも使えます。"},
			},
		},
		{
			Name:     "first_play",
			Legend:   "1 ゲーム目の始め方",
			Selected: o.FirstPlay,
			Choices: []Choice{
				{"diamond3_must", "♦3 を必ず 1 枚出す", "♦3 を持っている人が、♦3 の単騎から始めます。"},
				{"diamond3_free", "♦3 を持つ人が自由に出す", "先手は ♦3 を持っている人ですが、出すカードは自由です。"},
				{"free", "席順の先頭から自由に出す", "♦3 に関係なく始めます。"},
			},
		},
		{
			Name:     "eight_cut",
			Legend:   "8流し (8切り)",
			Selected: o.EightCut,
			Choices: []Choice{
				{"off", "なし", "8 は普通のカードとして扱います。"},
				{"all", "あり (階段の 8 も含む)", "8 を出すと場が流れ、出した人が次の親になります。"},
				{"group_only", "あり (階段の 8 は対象外)", "同じ数字の出し方でだけ 8流しが起きます。"},
			},
		},
		{
			Name:     "jack_back",
			Legend:   "Jバック (イレブンバック)",
			Selected: o.JackBack,
			Choices: []Choice{
				{"off", "なし", "J は普通のカードとして扱います。"},
				{"all", "あり (階段の J も含む)", "J が出てから場が流れるまで、強弱が反転します。"},
				{"group_only", "あり (階段の J は対象外)", "同じ数字の出し方でだけ Jバックが起きます。"},
			},
			TogglesNote:   "Jバックを使うときだけ効きます。",
			TogglesHideOn: "off",
			Toggles: []Toggle{
				{
					Name:        "jack_back_optional",
					Label:       "任意適用にする",
					Description: "J を出した人が、反転させるかどうかをその場で決めます。外すと強制適用です。",
					On:          o.JackBackMode == "optional",
				},
			},
		},
		{
			Name:     "seq_revolution",
			Legend:   "階段革命",
			Selected: o.SeqRevolution,
			Choices: []Choice{
				{"off", "なし", "革命はフォーカード (同じ数字 4 枚以上) だけです。"},
				{"on", "あり (4 枚以上の階段)", "4 枚以上の階段でも革命が起きます。"},
			},
		},
		{
			Name:     "five_card",
			Legend:   "ファイブカード・シックスカード",
			Selected: o.FiveCard,
			Choices: []Choice{
				{"off", "なし", "同じ数字は 4 枚までしか出せません。"},
				{"on", "あり", "ジョーカーを混ぜて同じ数字を 5 枚・6 枚として出せます (革命にもなります)。"},
			},
		},
		{
			Name:     "bind",
			Legend:   "縛り",
			Selected: o.Bind,
			Choices: []Choice{
				{"off", "なし", "マークによる制限はありません。"},
				{"on", "あり", "1 つ前と同じマーク構成が出たら、以降そのマークを必ず含めて出します。"},
			},
			TogglesNote:   "どちらも「縛りの一種」です。縛りを使うときだけ効きます。両方を入れても構いません。",
			TogglesHideOn: "off",
			Toggles: []Toggle{
				{
					Name:        "bind_strict",
					Label:       "激縛り",
					Description: "縛りが発生したときに数字が 1 つ上がっていたら、次はその次の数字しか出せません。",
					On:          onOff(o.BindStrict),
				},
				{
					Name:        "bind_partial",
					Label:       "片方縛り",
					Description: "マークが一部だけ一致した場合にも、その一致したマークで縛ります。",
					On:          onOff(o.BindPartial),
				},
			},
		},
		{
			Name:     "ten_discard",
			Legend:   "10捨て (じゅっすて)",
			Selected: o.TenDiscard,
			Choices: []Choice{
				{"off", "なし", "10 は普通のカードとして扱います。"},
				{"on", "あり", "10 を出した枚数だけ、手札から要らないカードを捨てられます。"},
			},
		},
		{
			Name:     "seven_give",
			Legend:   "7渡し (ななわたし)",
			Selected: o.SevenGive,
			Choices: []Choice{
				{"off", "なし", "7 は普通のカードとして扱います。"},
				{"on", "あり", "7 を出した枚数だけ、手札から要らないカードを次の人へ渡します。"},
			},
		},
		{
			Name:     "spade_three",
			Legend:   "スペ3返し",
			Selected: o.SpadeThree,
			Choices: []Choice{
				{"off", "なし", "ジョーカーの単騎には誰も勝てません。"},
				{"on", "あり", "ジョーカー 1 枚に ♠3 1 枚で勝てます。場は流れ、♠3 を出した人が親になります。"},
			},
		},
		{
			Name:     "pass_lock",
			Legend:   "パスの処理",
			Selected: o.PassLock,
			Choices: []Choice{
				{"off", "何度でも出せる", "パスしても、自分の番が回ってくればまた出せます。"},
				{"on", "一度パスしたら流れるまで出せない", "場が流れるまで、その人は手を出せません。"},
			},
		},
		{
			Name:     "finish_flush",
			Legend:   "誰かが上がったときの場の扱い",
			Selected: o.FinishFlush,
			Choices: []Choice{
				{"off", "そのまま続ける", "場に残ったカードに対して、次の人から続けて出します。"},
				{"on", "上がった瞬間に流す", "場を流して、上がった人の次の人が親になります。"},
			},
		},
		{
			Name:     "foul_finish",
			Legend:   "反則上がり禁止",
			Selected: o.FoulFinish,
			Choices: []Choice{
				{"off", "なし", "どのカードで上がっても構いません。"},
				{"on", "あり", "ジョーカー・通常時の 2・革命時の 3 (採用していれば 8 と J) で上がると大貧民になります。10捨て・7渡しで手札が足りないときも反則です。"},
			},
		},
		{
			Name:     "miyako_ochi",
			Legend:   "都落ち",
			Selected: o.MiyakoOchi,
			Choices: []Choice{
				{"off", "なし", "前回の大富豪も普通に順位を争います。"},
				{"on", "あり", "前のゲームの大富豪が 1 位を守れなかった時点で、手札を捨てて大貧民になります。"},
			},
		},
	}
}

// ParseOptions はフォームの値を Options にする。
// 知らない値は既定に落とす (画面から来る値なので厳密に弾く必要はない)。
//
// チェックボックスは、入っていれば値が送られ、外れていれば何も送られない。
// 「外した」と「そもそも送られていない」を区別できないので、
// チェックボックスはいったん全て off にしてから、届いたものだけ on にする。
func ParseOptions(get func(string) string) Options {
	o := DefaultOptions()
	// チェックボックスの既定は全て off にしてから、送られてきたものだけ on にする。
	o.JackBackMode = "forced"
	o.BindStrict = "off"
	o.BindPartial = "off"

	for _, g := range o.Groups() {
		if v := get(g.Name); validChoice(g, v) {
			o.setChoice(g.Name, v)
		}
		for _, t := range g.Toggles {
			if get(t.Name) != "" {
				o.setToggle(t.Name)
			}
		}
	}
	return o
}

func (o *Options) setChoice(name, v string) {
	switch name {
	case "players":
		o.Players = atoiOr(v, o.Players)
	case "jokers":
		o.Jokers = atoiOr(v, o.Jokers)
	case "first_play":
		o.FirstPlay = v
	case "eight_cut":
		o.EightCut = v
	case "jack_back":
		o.JackBack = v
	case "seq_revolution":
		o.SeqRevolution = v
	case "five_card":
		o.FiveCard = v
	case "bind":
		o.Bind = v
	case "ten_discard":
		o.TenDiscard = v
	case "seven_give":
		o.SevenGive = v
	case "spade_three":
		o.SpadeThree = v
	case "pass_lock":
		o.PassLock = v
	case "finish_flush":
		o.FinishFlush = v
	case "foul_finish":
		o.FoulFinish = v
	case "miyako_ochi":
		o.MiyakoOchi = v
	}
}

func (o *Options) setToggle(name string) {
	switch name {
	case "jack_back_optional":
		o.JackBackMode = "optional"
	case "bind_strict":
		o.BindStrict = "on"
	case "bind_partial":
		o.BindPartial = "on"
	}
}

func validChoice(g Group, v string) bool {
	for _, c := range g.Choices {
		if c.Value == v {
			return true
		}
	}
	return false
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// Labels は選んだ流儀の短い説明を並べる。盤面の上に小さく出す。
// 使っていない流儀 (縛りなしのときの激縛りなど) は出さない。
func (o Options) Labels() []string {
	var out []string
	for _, g := range o.Groups() {
		for _, c := range g.Choices {
			if c.Value != g.Selected {
				continue
			}
			switch g.Name {
			case "players", "jokers":
				out = append(out, c.Label)
			default:
				out = append(out, g.Legend+": "+c.Label)
			}
		}
		if g.Selected == "off" {
			continue // 使わない流儀の枝分かれは出さない
		}
		for _, t := range g.Toggles {
			if t.On {
				out = append(out, "　+ "+t.Label)
			}
		}
	}
	return out
}

// =============================================
// プリセット
// =============================================

// Preset は設定画面に出す、ローカルルールの組み合わせ 1 つぶん。
type Preset struct {
	Name        string // フォームの値
	Label       string
	Description string
}

// プリセットの名前。
const (
	PresetBasic       = "basic"
	PresetRecommended = "recommended"
)

// Presets は設定画面に並べるプリセット。
func Presets() []Preset {
	return []Preset{
		{PresetBasic, "基本ルールのみ", "ローカルルールをすべて外します。説明書の基本ルールだけで遊べます。"},
		{PresetRecommended, "作者オススメ", "8流し・Jバック・縛り・10捨てなどを盛った、賑やかな卓です。"},
	}
}

// BasicOptions はローカルルールをすべて外した設定。
// ジョーカー 2 枚と「♦3 を持つ人が自由に出す」は基本ルールの側に含める。
func BasicOptions() Options {
	return Options{
		Players:       4,
		Jokers:        2,
		EightCut:      "off",
		JackBack:      "off",
		JackBackMode:  "forced",
		SeqRevolution: "off",
		FiveCard:      "off",
		Bind:          "off",
		BindStrict:    "off",
		BindPartial:   "off",
		TenDiscard:    "off",
		SevenGive:     "off",
		SpadeThree:    "off",
		PassLock:      "off",
		FinishFlush:   "off",
		FoulFinish:    "off",
		MiyakoOchi:    "off",
		FirstPlay:     "diamond3_free",
	}
}

// presetOptions は名前からプリセットの設定を引く。
func presetOptions(name string) (Options, bool) {
	switch name {
	case PresetBasic:
		return BasicOptions(), true
	case PresetRecommended:
		return DefaultOptions(), true
	}
	return Options{}, false
}

// WithPreset はローカルルールをプリセットに置き換える。
// 人数はローカルルールではないので、今の値を残す。
// 知らない名前なら元のまま返す。
func (o Options) WithPreset(name string) Options {
	p, ok := presetOptions(name)
	if !ok {
		return o
	}
	p.Players = o.Players
	return p
}

// PresetName は今の設定と一致するプリセットの名前を返す。無ければ空。
// 人数と、親の流儀を使わないときの枝分かれは比べない。
func (o Options) PresetName() string {
	for _, p := range Presets() {
		want, _ := presetOptions(p.Name)
		want.Players = o.Players
		if want.normalized() == o.normalized() {
			return p.Name
		}
	}
	return ""
}

// normalized は使っていない流儀の枝分かれを既定に揃える。
// Jバックなしのときの「任意適用」のように、効かない値の違いで
// 「別の設定」と見なさないため。
func (o Options) normalized() Options {
	if o.JackBack == "off" {
		o.JackBackMode = "forced"
	}
	if o.Bind == "off" {
		o.BindStrict = "off"
		o.BindPartial = "off"
	}
	return o
}
