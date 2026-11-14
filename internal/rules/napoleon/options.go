package napoleon

// options.go — ゲーム開始前に選ぶローカルルール。
//
// 判定そのものは Prolog 側 (napoleon_rules.pl の動的述語) が持つ。
// ここにあるのは「どの値を選べるか」と「画面に出す説明文」だけ。

import "fmt"

// Options は選べるローカルルール一式。値は全て Prolog 側のアトムと同じ文字列。
type Options struct {
	Players      int    // 人数 4/5/6
	MinBid       int    // 最低宣言枚数 10/12/14
	SameTwo      string // off / on / not_trump
	Yoromeki     string // off / on / return
	JokerRule    string // none / top / second / below / bottom
	JokerFree    string // off / on
	JokerLead    string // none / trump
	JokerDemand  string // off / on
	MightyExempt string // off / on
	Scoring      string // twelfth / wikipedia
	Slam         string // normal / double / reverse
}

// DefaultOptions は既定のローカルルール。
// 本の基本ルールに、よく遊ばれているローカルルール
// (セイムツー・よろめき・ジョーカー・三役の拒否権) を足した形。
func DefaultOptions() Options {
	return Options{
		Players:      5,
		MinBid:       10,
		SameTwo:      "not_trump",
		Yoromeki:     "on",
		JokerRule:    "second",
		JokerFree:    "off",
		JokerLead:    "trump",
		JokerDemand:  "off",
		MightyExempt: "on",
		Scoring:      "twelfth",
		Slam:         "normal",
	}
}

// Apply は Prolog 側の設定を書き換える。
func (o Options) Apply(e *Engine) error {
	steps := []struct {
		query string
		what  string
	}{
		{fmt.Sprintf("set_players(%d).", o.Players), "人数"},
		{fmt.Sprintf("set_min_bid(%d).", o.MinBid), "最低宣言枚数"},
		{fmt.Sprintf("set_same_two(%s).", o.SameTwo), "セイムツー"},
		{fmt.Sprintf("set_yoromeki(%s).", o.Yoromeki), "よろめき"},
		{fmt.Sprintf("set_joker_rule(%s).", o.JokerRule), "ジョーカー"},
		{fmt.Sprintf("set_joker_free(%s).", o.JokerFree), "ジョーカーの自由出し"},
		{fmt.Sprintf("set_joker_lead(%s).", o.JokerLead), "切り札請求"},
		{fmt.Sprintf("set_joker_demand(%s).", o.JokerDemand), "ジョーカー請求"},
		{fmt.Sprintf("set_mighty_exempt(%s).", o.MightyExempt), "三役の拒否権"},
		{fmt.Sprintf("set_scoring_rule(%s).", o.Scoring), "得点表"},
		{fmt.Sprintf("set_slam_rule(%s).", o.Slam), "20 枚独占の扱い"},
	}
	for _, s := range steps {
		if err := e.p.Set(s.query, s.what); err != nil {
			return err
		}
	}
	return nil
}

// UsesJoker はジョーカーを入れる設定か。
func (o Options) UsesJoker() bool { return o.JokerRule != "none" }

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
// 「セイムツーあり」に対する「切り札では無効」のように、選んだ流儀を
// さらに細かくする枝分かれを表す。
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
	// TogglesNote は Toggles の上に出す注釈。
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
//
// セイムツーとよろめきは Prolog 側では 3 値だが、画面では
// 「なし / あり」のラジオボタンと、「あり」のときだけ出る枝分かれの
// チェックボックスに分けて見せる。
func (o Options) Groups() []Group {
	return []Group{
		{
			Name:     "players",
			Legend:   "人数",
			Selected: fmt.Sprint(o.Players),
			Choices: []Choice{
				{"5", "5人 (基本)", "10 枚ずつ配り、残りを場に伏せます。"},
				{"4", "4人", "12 枚ずつ配ります。副官ありなら 2 対 2 です。"},
				{"6", "6人", "8 枚ずつ配ります。連合軍が厚くなります。"},
			},
		},
		{
			Name:     "min_bid",
			Legend:   "最低宣言枚数",
			Selected: fmt.Sprint(o.MinBid),
			Choices: []Choice{
				{"10", "10 枚から", "絵札 20 枚のちょうど半分。競りが短く済みます。"},
				{"12", "12 枚から", "ナポレオン軍に厳しい設定です。"},
				{"14", "14 枚から", "さらに厳しい設定です。"},
			},
		},
		{
			Name:     "same_two",
			Legend:   "セイムツー",
			Selected: offOn(o.SameTwo != "off"),
			Choices: []Choice{
				{"off", "なし", "2 は数字どおり最弱のままです。"},
				{"on", "あり", "台札と同じマークだけが出揃い、その中に 2 があればその 2 が勝ちます。"},
			},
			Toggles: []Toggle{
				{
					Name:        "same_two_not_trump",
					Label:       "切り札では無効",
					Description: "台札が切り札のマークのときは発動しません。",
					On:          o.SameTwo == "not_trump",
				},
			},
			TogglesHideOn: "off",
		},
		{
			Name:     "yoromeki",
			Legend:   "よろめき",
			Selected: offOn(o.Yoromeki != "off"),
			Choices: []Choice{
				{"off", "なし", "♠A (オールマイティ) はそのまま最強です。"},
				{"on", "あり", "♠A と ♥Q が同じトリックに出たら ♥Q が勝ちます。"},
			},
			Toggles: []Toggle{
				{
					Name:        "yoromeki_return",
					Label:       "よろめき返し",
					Description: "さらに ♥K もいたら ♥K が勝ちます。",
					On:          o.Yoromeki == "return",
				},
			},
			TogglesHideOn: "off",
		},
		{
			Name:     "joker_rule",
			Legend:   "ジョーカー",
			Selected: o.JokerRule,
			Choices: []Choice{
				{"none", "入れない (52 枚)", "本の基本ルールです。"},
				{"second", "入れる / 2 番手", "♠A > ジョーカー > 正J > 裏J。"},
				{"top", "入れる / 最強", "ジョーカー > ♠A > 正J > 裏J。"},
				{"below", "入れる / 三役未満", "♠A > 正J > 裏J > ジョーカー > 切り札。"},
				{"bottom", "入れる / 最弱", "どのカードにも勝てません。自由出しと組み合わせると使い道が出ます。"},
			},
			Toggles: []Toggle{
				{
					Name:        "joker_free",
					Label:       "ジョーカーの自由出し",
					Description: "台札に関係なく、いつでもジョーカーを出せます。",
					On:          o.JokerFree == "on",
				},
				{
					Name:        "joker_lead",
					Label:       "切り札請求 (ジョーカーが台札のとき)",
					Description: "ジョーカーが台札なら、切り札を持っている人は必ず出します。",
					On:          o.JokerLead == "trump",
				},
				{
					Name:        "joker_demand",
					Label:       "ジョーカー請求 (♠3 が台札のとき)",
					Description: "♠3 が台札なら、ジョーカーを持っている人は必ず出します。",
					On:          o.JokerDemand == "on",
				},
			},
			TogglesNote:   "ジョーカーを入れるときだけ効きます。",
			TogglesHideOn: "none",
		},
		{
			Name:     "mighty_exempt",
			Legend:   "三役の拒否権",
			Selected: o.MightyExempt,
			Choices: []Choice{
				{"off", "なし", "三役も台札のマークなら出さなければなりません。"},
				{"on", "あり", "台札のマークが三役しかないときは、出さずに温存できます。"},
			},
		},
		{
			Name:     "scoring",
			Legend:   "宣言枚数による得点の重み",
			Selected: o.Scoring,
			Choices: []Choice{
				{"twelfth", "なし", "宣言枚数によらず、ナポレオン ±2 (副官なし ±4)、副官 ±1、連合軍 ∓1。"},
				{"wikipedia", "あり", "連合軍の点 = 宣言枚数 −(最低枚数−1)。ナポレオンはその 2 倍 (副官なしは 4 倍)。"},
			},
		},
		{
			Name:     "slam",
			Legend:   "20 枚独占したとき",
			Selected: o.Slam,
			Choices: []Choice{
				{"normal", "普通に数える", "特別扱いしません。"},
				{"double", "得点が 2 倍", "取った側の勝ちのまま、点数だけ 2 倍になります。"},
				{"reverse", "取った側の負け + 2 倍", "20 枚すべて取った方が負けになり、点数も 2 倍です。"},
			},
		},
	}
}

// offOn は画面のラジオボタン用に、使うかどうかを on/off にする。
func offOn(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// ParseOptions はフォームの値を Options にする。
// 知らない値は既定に落とす (画面から来る値なので厳密に弾く必要はない)。
//
// チェックボックスは、入っていれば値が送られ、外れていれば何も送られない。
// 「外した」と「そもそも送られていない」を区別できないので、
// チェックボックスで決まる値はいったん全て off にしてから、
// 届いたものだけ on にする。
func ParseOptions(get func(string) string) Options {
	o := DefaultOptions()
	o.SameTwo = "off"
	o.Yoromeki = "off"
	o.JokerFree = "off"
	o.JokerLead = "none"
	o.JokerDemand = "off"

	for _, g := range o.Groups() {
		selected := g.Selected
		if v := get(g.Name); validChoice(g, v) {
			o.setChoice(g.Name, v)
			selected = v
		}
		if g.TogglesHideOn != "" && selected == g.TogglesHideOn {
			continue // 使わない流儀の枝分かれは、届いても無視する
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
	case "min_bid":
		o.MinBid = atoiOr(v, o.MinBid)
	case "same_two":
		o.SameTwo = v
	case "yoromeki":
		o.Yoromeki = v
	case "joker_rule":
		o.JokerRule = v
	case "mighty_exempt":
		o.MightyExempt = v
	case "scoring":
		o.Scoring = v
	case "slam":
		o.Slam = v
	}
}

func (o *Options) setToggle(name string) {
	switch name {
	case "same_two_not_trump":
		o.SameTwo = "not_trump"
	case "yoromeki_return":
		o.Yoromeki = "return"
	case "joker_free":
		o.JokerFree = "on"
	case "joker_lead":
		o.JokerLead = "trump"
	case "joker_demand":
		o.JokerDemand = "on"
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
// 使っていない流儀 (ジョーカーを入れないときの自由出しなど) は出さない。
func (o Options) Labels() []string {
	var out []string
	for _, g := range o.Groups() {
		for _, c := range g.Choices {
			if c.Value != g.Selected {
				continue
			}
			switch g.Name {
			case "players", "min_bid":
				out = append(out, c.Label)
			default:
				out = append(out, g.Legend+": "+c.Label)
			}
		}
		if g.TogglesHidden() {
			continue
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
		{PresetRecommended, "作者オススメ", "ジョーカー・セイムツー・よろめき・三役の拒否権を入れた形です。"},
	}
}

// BasicOptions はローカルルールをすべて外した設定 (52 枚、特殊な勝ち方なし)。
func BasicOptions() Options {
	return Options{
		Players:      5,
		MinBid:       10,
		SameTwo:      "off",
		Yoromeki:     "off",
		JokerRule:    "none",
		JokerFree:    "off",
		JokerLead:    "none",
		JokerDemand:  "off",
		MightyExempt: "off",
		Scoring:      "twelfth",
		Slam:         "normal",
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
// 人数と、ジョーカーを入れないときの枝分かれは比べない。
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
func (o Options) normalized() Options {
	if o.JokerRule == "none" {
		o.JokerFree = "off"
		o.JokerLead = "none"
		o.JokerDemand = "off"
	}
	return o
}
