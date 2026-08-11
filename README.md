# ハーツ (Hearts) — Prolog ルール + Go の CUI

CPU 3 人と対戦できるハーツ。**ゲームのルール判定はすべて Prolog 側**にあり、
Go は純 Go 実装の Prolog 処理系 [ichiban/prolog](https://github.com/ichiban/prolog)
にルールファイルを読み込ませて、シャッフル・画面表示・入力だけを担当する。

## 遊び方

```
go build -o hearts.exe .
./hearts.exe
```

| オプション | 意味 |
| --- | --- |
| `-limit 100` | この失点に達したらゲーム終了 |
| `-players 4` | 人数 3 / 4 / 5 (hearts.pl の `game_config/4` に対応) |
| `-seed 42` | 乱数シード (同じ配札を再現できる) |
| `-demo` | あなたの席も CPU が打つ自動再生 |
| `-suit auto` | スートの表示 (下記) |
| `-nocolor` | 色を使わない |
| `-pl .` | 埋め込みではなくこのディレクトリの `.pl` を読む |

`-pl .` を付ければ **再ビルドせずに `.pl` を書き換えて挙動を試せる**。

### `-suit` — スートの表示

`♠♥♦♣` は East Asian **Ambiguous** 幅の文字で、端末によって 1 桁にも 2 桁にも
描画される。桁揃えの前提と実際の描画が食い違うと、手札の列が重なって見える。

| 値 | 動作 |
| --- | --- |
| `auto` (既定) | Windows では `letter`、それ以外では `symbol` |
| `letter` | `S/H/D/C` で表示。全部 ASCII なので**どの端末でも桁がずれない** |
| `symbol` | `♠♥♦♣` を 1 桁として桁揃え |
| `wide` | `♠♥♦♣` を 2 桁として桁揃え |

PowerShell / Windows コンソールで記号がずれる場合は `letter` (既定) のまま使うか、
`-suit symbol` と `-suit wide` を試してその端末に合う方を選べばよい。

## ファイル構成

### Prolog (ルール)

| ファイル | 役割 |
| --- | --- |
| `hearts.pl` | **元のまま。1 行も変更していない。** カードの強さ、点数 (`card_point/2`)、マストフォロー (`can_play/3`)、トリックの勝者 (`determine_winner/3`)、採点 (`calculate_score/2`)、配札の検証 (`validate_deal/1`)、人数設定 (`game_config/4`) |
| `compat.pl` | `hearts.pl` を無修正で読むための互換レイヤ。ichiban/prolog は ISO 準拠なので、SWI 固有の `:- dynamic` 前置演算子や `include/3`, `flatten/2`, `is_set/1` などをここで補う |
| `hearts_rules.pl` | `hearts.pl` の述語を**呼び出して**組み立てた追加ルール。デッキ生成、♣2 スタート、第 1 トリックの点数札禁止、ハートブレイク、カード交換 (パス)、スラム、終了判定 |
| `hearts_ai.pl` | CPU の思考。`rank_value/2`, `is_stronger/2`, `is_suit/2`, `determine_winner/3` をそのまま使ってカードを選ぶ |

読み込み順は `compat.pl → hearts.pl → hearts_rules.pl → hearts_ai.pl`。

### Go

| ファイル | 役割 |
| --- | --- |
| `rules.go` | Prolog ブリッジ。`card(Suit, Rank)` ↔ Go の `Card` 変換と、各述語への問い合わせ |
| `game.go` | 進行管理 (配札・パス・13 トリック・精算)。ルール判定は書かず、すべて `Rules` 経由で Prolog に尋ねる |
| `ui.go` | 手札や場の描画、入力 |
| `main.go` | フラグ処理 |
| `console_windows.go` | Windows コンソールを UTF-8 + ANSI 対応に切り替える (日本語とスート記号の文字化け対策) |
| `rules_test.go` | Prolog 側ルールの検証 (`go test ./...`) |

## 実装されているルール

- ♥ 1 枚 = 1 点、♠Q = 13 点。失点が少ないほど良い
- 台札と同じスートを持っていたら必ず出す (マストフォロー)
- 第 1 トリックは ♣2 スタート、点数札は出せない
- ハートが場に出るまでハートからリードできない (手札がハートだけなら例外)
- 毎ラウンド 3 枚交換 (左 → 右 → 向かい → 交換なし の 4 周期。奇数人数では左 → 右 → 交換なし)
- スラム (1 人で全 26 点を独占 → 本人 0 点、他全員 26 点)
- 誰かが上限点に達したら終了、最少失点が勝ち

3 人戦では ♣2 を除いた 51 枚 17 枚ずつ、5 人戦では ♣2 と ♦2 を除いた 50 枚 10 枚ずつ
(`hearts.pl` の `game_config/4` どおり)。開始カードは ♣2 が無い人数では ♣3 になる。
