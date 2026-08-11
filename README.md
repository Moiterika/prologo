# ハート (Hearts) — Prolog ルール + GoTH (Go / templ / htmx)

CPU 3 人と対戦できるハート。**ゲームのルール判定はすべて Prolog 側**にあり、
Go は純 Go 実装の Prolog 処理系 [ichiban/prolog](https://github.com/ichiban/prolog)
にルールファイルを読み込ませて、シャッフル・進行管理・画面の描画だけを担当する。

画面は templ + htmx。WebSocket もデータベースも使わない。
ブラウザからの POST でゲームを 1 手進め、その場で描き直して返すだけ。

## 遊び方

```
go build -o hearts.exe ./cmd/game
./hearts.exe
```

起動したら http://127.0.0.1:8080/ を開く。

`Makefile` も置いてあるが (`make run` / `make test` / `make doc-http` …)、
`make` が入っていない環境でも困らないよう、以下では素の `go` コマンドを併記する。
括弧内が対応する make のターゲット。

| オプション | 意味 |
| --- | --- |
| `-addr 127.0.0.1:8080` | 待ち受けアドレス |
| `-limit 100` | この失点に達したらゲーム終了 |
| `-players 4` | 人数 3 / 4 / 5 (hearts.pl の `game_config/4` に対応) |
| `-seed 42` | 乱数シード (同じ配札を再現できる) |
| `-pl internal/rules` | 埋め込みではなくこのディレクトリの `.pl` を読む |

`-pl internal/rules` を付ければ **再ビルドせずに `.pl` を書き換えて挙動を試せる**。

`.templ` を編集したときは `go tool templ generate` (make generate)。
生成物の `*_templ.go` はコミットしてあるので、通常のビルドでは不要。

## ディレクトリ構成

```
cmd/game/            エントリポイント (フラグ処理とサーバー起動)
internal/rules/      ルール判定。Prolog ブリッジと .pl 本体
internal/game/       1 局分の進行 (状態機械)
internal/web/        HTTP ハンドラとセッション
internal/web/view/   templ による画面
```

依存の向きは `cmd → web → game → rules` の一方向。
`rules` は Go の世界のことも画面のことも知らず、`game` は画面も HTTP も知らない。

### internal/rules — ルール判定

| ファイル | 役割 |
| --- | --- |
| `hearts.pl` | **元のまま。1 行も変更していない。** カードの強さ、点数 (`card_point/2`)、マストフォロー (`can_play/3`)、トリックの勝者 (`determine_winner/3`)、採点 (`calculate_score/2`)、配札の検証 (`validate_deal/1`)、人数設定 (`game_config/4`) |
| `compat.pl` | `hearts.pl` を無修正で読むための互換レイヤ。ichiban/prolog は ISO 準拠なので、SWI 固有の `:- dynamic` 前置演算子や `include/3`, `flatten/2`, `is_set/1` などをここで補う |
| `hearts_rules.pl` | `hearts.pl` の述語を**呼び出して**組み立てた追加ルール。デッキ生成、第 1 トリックの流儀 (`first_trick_points/1`, `first_trick_lead/1`)、ハートブレイク (`hearts_break_rule/1`)、カード交換 (パス)、スラム、終了判定 |
| `hearts_ai.pl` | CPU の思考。`rank_value/2`, `is_stronger/2`, `is_suit/2`, `determine_winner/3` をそのまま使ってカードを選ぶ |
| `rules.go` | `rules.Engine`。`card(Suit, Rank)` ↔ Go の `Card` 変換と各述語への問い合わせ |
| `options.go` | 開始時に選ぶ流儀。Prolog 側の設定と画面に出す説明文 |
| `label.go` | ♠♥♦♣ などの表示用ラベル |

読み込み順は `compat.pl → hearts.pl → hearts_rules.pl → hearts_ai.pl`。
`.pl` は `go:embed` で実行ファイルに埋め込むため、埋め込む側の
`internal/rules` に置いてある (embed は親ディレクトリを参照できない)。

`rules.Engine` は並行安全ではない。対局ごとに 1 つ作ること。
使い方は [ドキュメント](#ドキュメント)の Example を参照。

### internal/game — 進行

`game.Game` が状態機械。画面も HTTP も知らず、ルール判定も自分では書かずに
すべて `rules.Engine` 経由で Prolog に尋ねる。局面は
`passing` / `playing` / `trick_done` / `round_done` / `game_over` の 5 つ。

プレイヤーの入力ミスは `game.ErrPlayerInput` で返し、web 側が 500 ではなく
画面上のメッセージとして出す。

### internal/web — 画面

- **htmx**: `<body>` に `hx-boost` を付けてあるので、テンプレート側は素の
  `<form method="post">` を書くだけでよい。応答から `#app` を取り出して差し替える。
  JavaScript を切っても普通のフォーム送信としてそのまま遊べる。
- **`hx-push-url="false"`**: 既定だと POST 先の URL がアドレス欄に載り、
  そこでリロードすると `GET /play` になってしまう。ゲームの状態はすべて
  サーバー側にあり履歴に意味が無いので URL は `/` に固定する。
  (念のため `GET /play` などは `/` へリダイレクトする)
- **セッション**: Cookie の ID でプロセス内のメモリに紐付ける。
  Prolog インタプリタもセッションごとに 1 つ持つので、同時アクセスで状態が混ざらない。
- **円卓**: 3x3 のグリッドに席を置き、「あなた」を必ず手前 (下段中央) にする。
  出したカードは名札の中央寄りに並ぶので、誰が出した札かが分かる。

## ドキュメント

端末に出す場合。

```
go doc -all ./internal/rules      # Example つきの全文    (make doc)
go doc ./internal/rules Engine    # 型だけ
```

ブラウザで見る場合。`go` ツールチェーンにドキュメントサーバーは同梱されていない
(Go 1.13 で `godoc` が本体から外れて `golang.org/x/tools` へ移った) ので、
`go run` で取ってくる。`@latest` なので `go.mod` の依存には入らない。

```
go run golang.org/x/tools/cmd/godoc@latest -http 127.0.0.1:6060    # (make doc-http)
```

起動後、数秒待ってから http://127.0.0.1:6060/pkg/hearts/internal/rules/ を開く。
`internal/` パッケージもそのまま見えて、Example は `// Output:` の中身まで展開される。

### PowerShell から直接叩くときの注意

PowerShell 5.1 は `=` を含む引数を**勝手に分割する**。

```
> go run . -http=127.0.0.1:6060
argv[1] = "-http=127"
argv[2] = ".0.0.1:6060"        ← 2 つに割れて「Unexpected arguments」になる
```

クォートするか、スペース区切りにすれば通る。

```powershell
go run golang.org/x/tools/cmd/godoc@latest "-http=127.0.0.1:6060"
go run golang.org/x/tools/cmd/godoc@latest -http 127.0.0.1:6060
```

`hearts.exe` のフラグも同じなので、`-addr=...` ではなく `-addr ...` と書く。
`make` 経由なら sh が解釈するのでこの問題は起きない。

### pkgsite (pkg.go.dev と同じ見た目) を使う場合

現行の公式ツールは [pkgsite](https://pkg.go.dev/golang.org/x/pkgsite) だが、
**このモジュールはそのままでは表示できない**。モジュールパス `hearts` は最初の要素に
ドットが無いため、pkgsite が標準ライブラリのパスと解釈して 424 を返す。

```
go run golang.org/x/pkgsite/cmd/pkgsite@latest -http 127.0.0.1:8090 .
```

使うなら `go.mod` のモジュールパスを `example.com/hearts` のような
ドットを含むものに変える必要がある (公開する予定ができたときにリポジトリの URL へ)。

## 実装されているルール

- ♥ 1 枚 = 1 点、♠Q = 13 点。失点が少ないほど良い
- 台札と同じスートを持っていたら必ず出す (マストフォロー)
- 毎ラウンド 3 枚交換 (左 → 右 → 向かい → 交換なし の 4 周期。奇数人数では左 → 右 → 交換なし)
- スラム (1 人で全 26 点を独占 → 本人 0 点、他全員 26 点)
- 誰かが上限点に達したら終了、最少失点が勝ち

### 開始時に選べる流儀

流儀が分かれる部分は、ゲーム開始前の画面で選ぶ。判定は Prolog 側
(`hearts_rules.pl` の `first_trick_points/1`, `first_trick_lead/1`,
`hearts_break_rule/1`) が持つ。

**第1トリックのリード**

| 選択 | 動作 |
| --- | --- |
| ♣2 から始める (既定) | ♣2 を持つ人が、必ずそのカードでリードする |
| ♣2 を持つ人が自由に出せる | ♣2 を持つ人が先手になるが、出すカードは自由 |

**第1トリックに点数札を出せるか**

| 選択 | 動作 |
| --- | --- |
| 点数札許可 (既定) | 制限なし。台札のスートが無ければハートも ♠Q も捨てられる |
| ♥ のみ禁止 | ハートは出せないが、台札のスートが無ければ ♠Q は捨てられる |
| 点数札禁止 | ♥ 全般と ♠Q を出せない。いわゆる no blood on the first trick |

どの流儀でも、**禁止されたカードしか持っていない場合は出さざるを得ないので許される**。
「禁止されたカード」が何かは流儀で変わる点に注意。たとえば「♥ のみ禁止」で
手札が `♥3 ♠Q` なら、♠Q が出せるのでハートは禁止のまま。

**ハートブレイク**

| 選択 | 動作 |
| --- | --- |
| なし (既定) | いつでもハートでリードできる |
| あり | **得点札 (♥ 全般と ♠Q) が場に出るまで**ハートからリードできない。手札がハートだけなら例外 |

ブレイクの引き金は「ハートが出たとき」ではなく「得点札が出たとき」なので、
♠Q が出ただけでもブレイクする (`hearts_rules.pl` の `contains_point_card/1`)。

「あり」を選んだときだけ、盤面にブレイクの状態表示と「ハートブレイク！」の知らせが出る。

3 人戦では ♣2 を除いた 51 枚 17 枚ずつ、5 人戦では ♣2 と ♦2 を除いた 50 枚 10 枚ずつ
(`hearts.pl` の `game_config/4` どおり)。開始カードは ♣2 が無い人数では ♣3 になる。

## テスト

```
go test -count=1 ./...    # (make test)
```

| ファイル | 内容 |
| --- | --- |
| `internal/rules/rules_test.go` | Prolog 側ルールの検証 (マストフォロー、第1トリックの各流儀、ハートブレイク、勝者判定、`validate_deal/1`、スラム、パス方向、AI が常に合法手を返すこと) |
| `internal/rules/example_test.go` | 使い方の実例 (Go の Example 関数)。マストフォローの判定とトリックの勝敗判定 |
| `internal/game/game_test.go` | 状態機械。自動操作で 1 ゲーム通し、局面遷移と失点の合計を確認 |
| `internal/web/viewmodel_test.go` | 円卓の席の割り当てと、出したカードがどの席に付くか |
| `internal/web/server_test.go` | HTTP 層。実際にサーバーを立ててフォーム送信でトリックを進め、セッションが独立していることを確認 |

Example 関数は `go test` で実行され、標準出力が末尾の `// Output:` コメントと
一致するかまで検証される。使い方の説明と回帰テストを兼ねる
(→ [ドキュメント](#ドキュメント))。

```
go test -run Example -v ./internal/rules
```
