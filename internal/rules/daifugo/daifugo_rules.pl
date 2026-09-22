% =============================================
% daifugo_rules.pl
%
% daifugo.pl の基本ルールの上に「ローカルルール」を重ねるレイヤ。
%
% 方針は hearts_rules.pl と同じで、daifugo.pl は一切変更せず、
% その述語 (classify/7, beats/4, card_strength/3, title/3, exchange/4 …) を
% 呼び出すことでルールを組み立てる。
%
% 切り替えられるローカルルールは全て動的述語で保持し、set_* で書き換える。
%
% 読み込み順: compat.pl -> daifugo.pl -> daifugo_rules.pl -> daifugo_ai.pl
% =============================================

% =============================================
% 1. 設定 (ゲーム開始前に選ぶローカルルール)
% =============================================

:- dynamic players/1.
:- dynamic joker_count/1.
:- dynamic eight_cut/1.
:- dynamic jack_back/1.
:- dynamic jack_back_mode/1.
:- dynamic seq_revolution/1.
:- dynamic five_card/1.
:- dynamic bind_rule/1.
:- dynamic bind_strict/1.
:- dynamic bind_partial/1.
:- dynamic ten_discard/1.
:- dynamic seven_give/1.
:- dynamic spade_three/1.
:- dynamic pass_lock/1.
:- dynamic finish_flush/1.
:- dynamic foul_finish_rule/1.
:- dynamic miyako_ochi/1.
:- dynamic first_play_rule/1.

% 人数 (3〜6)
players(4).

% ジョーカーの枚数 (0 / 1 / 2)
joker_count(2).

% 8流し (8切り)
%   off        … なし
%   all        … 8 を含むどの出し方でも流す
%   group_only … 同じ数字の出し方だけ。階段に含まれる 8 では流さない
eight_cut(all).

% Jバック (イレブンバック)。J が出てから流れるまで、強弱が擬似的に反転する
%   off / all / group_only  (8流しと同じ意味)
jack_back(off).

% Jバックの適用
%   forced   … J が出たら必ず反転する
%   optional … J を出した人が、反転させるかどうかを決める
jack_back_mode(forced).

% 階段革命。4 枚以上の階段を革命として扱うか
%   off / on
seq_revolution(off).

% ファイブカード・シックスカード。ジョーカーを混ぜて 5 枚以上の
% 同じ数字を出せるようにするか
%   off / on
five_card(off).

% 縛り。1 つ前と同じマーク構成が出たら、以降そのマークを必ず含める
%   off / on
bind_rule(off).

% 激縛り。縛りの一種で、縛り発生時に数字が連続していたら次の数字しか出せない。
% 縛り自体を使わない設定では意味を持たない。
%   off / on
bind_strict(off).

% 片方縛り。縛りの一種で、マークの一部が一致しただけで、そのマークで縛る。
% 縛り自体を使わない設定では意味を持たない。
%   off / on
bind_partial(off).

% 10捨て。10 を出した枚数だけ、手札から要らないカードを捨てられる
%   off / on
ten_discard(on).

% 7渡し。7 を出した枚数だけ、手札から要らないカードを次の人へ渡せる
%   off / on
seven_give(off).

% スペ3返し。単騎のジョーカーに ♠3 の単騎で勝てるか
%   off / on
spade_three(off).

% 一度パスしたら、場が流れるまでカードを出せないか
%   off / on
pass_lock(off).

% 誰かが上がった瞬間に場を流すか
%   off / on
finish_flush(off).

% 反則上がり。特定のカードで上がると大貧民になる
%   off / on
foul_finish_rule(off).

% 都落ち。開始時に大富豪だったプレイヤーが 1 位を維持できなければ大貧民
%   off / on
miyako_ochi(off).

% 1 ゲーム目の始め方
%   diamond3_must … ♦3 を持っている人が、必ず ♦3 単騎から始める
%   diamond3_free … ♦3 を持っている人が先手だが、出すカードは自由
%   free          … 席順の先頭から。出すカードも自由
first_play_rule(diamond3_must).

% --- 設定の書き換え ---
% 値の妥当性は member/2 で確かめる。画面から来た値をそのまま渡してよい。

set_players(N) :-
    integer(N),
    N >= 3,
    N =< 6,
    retractall(players(_)),
    assertz(players(N)).

set_joker_count(N) :-
    member(N, [0, 1, 2]),
    retractall(joker_count(_)),
    assertz(joker_count(N)).

set_eight_cut(M) :-
    member(M, [off, all, group_only]),
    retractall(eight_cut(_)),
    assertz(eight_cut(M)).

set_jack_back(M) :-
    member(M, [off, all, group_only]),
    retractall(jack_back(_)),
    assertz(jack_back(M)).

set_jack_back_mode(M) :-
    member(M, [forced, optional]),
    retractall(jack_back_mode(_)),
    assertz(jack_back_mode(M)).

set_seq_revolution(M) :-
    member(M, [off, on]),
    retractall(seq_revolution(_)),
    assertz(seq_revolution(M)).

set_five_card(M) :-
    member(M, [off, on]),
    retractall(five_card(_)),
    assertz(five_card(M)).

set_bind_rule(M) :-
    member(M, [off, on]),
    retractall(bind_rule(_)),
    assertz(bind_rule(M)).

set_bind_strict(M) :-
    member(M, [off, on]),
    retractall(bind_strict(_)),
    assertz(bind_strict(M)).

set_bind_partial(M) :-
    member(M, [off, on]),
    retractall(bind_partial(_)),
    assertz(bind_partial(M)).

set_ten_discard(M) :-
    member(M, [off, on]),
    retractall(ten_discard(_)),
    assertz(ten_discard(M)).

set_seven_give(M) :-
    member(M, [off, on]),
    retractall(seven_give(_)),
    assertz(seven_give(M)).

set_spade_three(M) :-
    member(M, [off, on]),
    retractall(spade_three(_)),
    assertz(spade_three(M)).

set_pass_lock(M) :-
    member(M, [off, on]),
    retractall(pass_lock(_)),
    assertz(pass_lock(M)).

set_finish_flush(M) :-
    member(M, [off, on]),
    retractall(finish_flush(_)),
    assertz(finish_flush(M)).

set_foul_finish_rule(M) :-
    member(M, [off, on]),
    retractall(foul_finish_rule(_)),
    assertz(foul_finish_rule(M)).

set_miyako_ochi(M) :-
    member(M, [off, on]),
    retractall(miyako_ochi(_)),
    assertz(miyako_ochi(M)).

set_first_play_rule(M) :-
    member(M, [diamond3_must, diamond3_free, free]),
    retractall(first_play_rule(_)),
    assertz(first_play_rule(M)).

% =============================================
% 2. デッキ (daifugo.pl の suit/1, rank/1 から生成)
% =============================================

deck(Deck) :-
    findall(card(S, R), ( suit(S), rank(R) ), Base),
    joker_count(J),
    joker_cards(J, Jokers),
    append(Base, Jokers, Deck).

joker_cards(0, []).
joker_cards(1, [card(joker, jk1)]).
joker_cards(2, [card(joker, jk1), card(joker, jk2)]).

% ゲーム開始カード (♦3)
opening_card(card(d, 3)).

% =============================================
% 3. 場 (フィールド) の表現
% =============================================
%   nil                                        … 場が空 (親番)
%   field(種別, 枚数, 強さ, マーク, ジョーカー数, 縛りマーク, 激縛り)
%
% 縛りマークは「以降の手が必ず含まねばならないマーク」の集合。
% 激縛りが true のときは、強さがちょうど +1 の手しか出せない。

field_form(field(Kind, Len, Strength, _, _, _, _), field_form(Kind, Len, Strength)).

field_suits(field(_, _, _, Suits, _, _, _), Suits).
field_bound(field(_, _, _, _, _, Bound, _), Bound).
field_strict(field(_, _, _, _, _, _, Strict), Strict).
field_strength(field(_, _, Strength, _, _, _, _), Strength).

% =============================================
% 4. 出し方が形として正しいか
% =============================================
% ファイブカード・シックスカードを使わない設定では、
% 同じ数字 5 枚以上を出せない。

form_allowed(group, Len) :-
    (   Len >= 5
    ->  five_card(on)
    ;   true
    ).
form_allowed(seq, _).

% =============================================
% 5. 合法手の判定
% =============================================
% resolve_play(手札, 出すカード, 場, 革命中か, 新しい場, 効果)
%
% 効果は effects(流す, 革命の切り替え, Jバックの発動) の形。
%
% ジョーカー入りの階段は解釈が複数あり得るので、
% 「場に勝てる解釈のうち、もっとも弱いもの」を採用する。
% こうしておくと、ジョーカーを足しただけで不必要に強い札とは見なされない。

resolve_play(Hand, Cards, Field, Rev, NewField, Effects) :-
    Cards = [_|_],
    subset(Cards, Hand),
    is_set(Cards),
    findall(Strength-form(Kind, Len, Suits, JokerN),
            ( classify(Cards, Rev, Kind, Len, Strength, Suits, JokerN),
              form_allowed(Kind, Len),
              playable(Cards, Kind, Len, Strength, Suits, JokerN, Field) ),
            Candidates),
    Candidates = [_|_],
    keysort(Candidates, Sorted),
    Sorted = [Strength-form(Kind, Len, Suits, JokerN)|_],
    next_field(Field, Kind, Len, Strength, Suits, JokerN, NewField),
    play_effects(Kind, Len, Cards, Field, Effects).

% can_play(手札, 出すカード, 場, 革命中か)
% 「出せるかどうか」だけを見る軽い版。どの解釈になるかは問わないので、
% 最初に見つかった時点で打ち切る。合法手の列挙 (daifugo_ai.pl) で使う。
can_play(Hand, Cards, Field, Rev) :-
    Cards = [_|_],
    subset(Cards, Hand),
    is_set(Cards),
    classify(Cards, Rev, Kind, Len, Strength, Suits, JokerN),
    form_allowed(Kind, Len),
    playable(Cards, Kind, Len, Strength, Suits, JokerN, Field),
    !.

% --- 場に出せるか ---
% 場が空なら形が正しいだけでよい。
playable(_, _, _, _, _, _, nil) :- !.
playable(Cards, Kind, Len, Strength, Suits, JokerN, Field) :-
    (   spade_three_return(Cards, Field)
    ->  true                                  % スペ3返しは例外
    ;   field_form(Field, Form),
        beats(Kind, Len, Strength, Form),
        bind_ok(Field, Suits, JokerN),
        strict_ok(Field, Strength)
    ).

% --- スペ3返し ---
% 単騎のジョーカーに、♠3 の単騎だけが勝てる。
spade_three_return(Cards, Field) :-
    spade_three(on),
    Cards == [card(s, 3)],
    Field = field(group, 1, 99, _, _, _, _).

% --- 縛り ---
% 縛られているマークのうち手に無いものは、ジョーカーで代用できる。
bind_ok(Field, Suits, JokerN) :-
    field_bound(Field, Bound),
    subtract(Bound, Suits, Missing),
    length(Missing, MissingN),
    MissingN =< JokerN.

% --- 激縛り ---
strict_ok(Field, Strength) :-
    field_strict(Field, Strict),
    (   Strict == true
    ->  field_strength(Field, FieldStrength),
        Strength =:= FieldStrength + 1
    ;   true
    ).

% =============================================
% 6. 出したあとの場の状態
% =============================================

next_field(nil, Kind, Len, Strength, Suits, JokerN,
           field(Kind, Len, Strength, Suits, JokerN, [], false)) :- !.
next_field(Field, Kind, Len, Strength, Suits, JokerN,
           field(Kind, Len, Strength, Suits, JokerN, Bound, Strict)) :-
    field_suits(Field, PrevSuits),
    field_bound(Field, PrevBound),
    field_strength(Field, PrevStrength),
    new_bind(PrevSuits, Suits, Fresh),
    append(PrevBound, Fresh, Bound0),
    list_to_set(Bound0, Bound),
    new_strict(Bound, PrevStrength, Strength, Strict).

% 縛りの発生。
%
% 激縛り (bind_strict) と片方縛り (bind_partial) は「縛りの一種」なので、
% 縛りそのものを使わない設定では効かない。この 2 つは互いに独立していて、
% 片方縛りが「どのマークで縛られるか」を、激縛りが「数字まで固定されるか」を
% 決める。両方を入れることもできる。
new_bind(PrevSuits, Suits, Bound) :-
    (   bind_rule(off)
    ->  Bound = []
    ;   bind_partial(on)
    ->  intersection(PrevSuits, Suits, Common),
        (   Common == []
        ->  Bound = []
        ;   Bound = Common
        )
    ;   same_suit_set(PrevSuits, Suits, Bound)
    ).

same_suit_set(PrevSuits, Suits, Bound) :-
    (   subset(PrevSuits, Suits),
        subset(Suits, PrevSuits),
        Suits \== []
    ->  Bound = Suits
    ;   Bound = []
    ).

% 激縛りの発生。縛りが効いていて、かつ数字がちょうど 1 つ上がったとき。
new_strict(Bound, PrevStrength, Strength, Strict) :-
    (   bind_rule(on),
        bind_strict(on),
        Bound \== [],
        Strength =:= PrevStrength + 1
    ->  Strict = true
    ;   Strict = false
    ).

% =============================================
% 7. 出したカードの効果 (8流し・革命・Jバック)
% =============================================

play_effects(Kind, Len, Cards, Field, effects(Cut, Revolution, JackBack)) :-
    (   cut_applies(Kind, Cards, Field) -> Cut = true ; Cut = false ),
    (   revolution_applies(Kind, Len) -> Revolution = true ; Revolution = false ),
    (   jack_back_applies(Kind, Cards) -> JackBack = true ; JackBack = false ).

% --- 8流し ---
cut_applies(Kind, Cards, _) :-
    eight_cut(Mode),
    Mode \== off,
    applies_to_kind(Mode, Kind),
    member(card(_, 8), Cards).
% スペ3返しも場を流す。♠3 を出した人が次の親になる。
cut_applies(_, Cards, Field) :-
    spade_three_return(Cards, Field).

% --- Jバック ---
jack_back_applies(Kind, Cards) :-
    jack_back(Mode),
    Mode \== off,
    applies_to_kind(Mode, Kind),
    member(card(_, j), Cards).

applies_to_kind(all, _).
applies_to_kind(group_only, group).

% --- 革命 ---
% フォーカード (同じ数字 4 枚以上) で革命。
% 階段革命を使う設定なら、4 枚以上の階段でも革命になる。
revolution_applies(group, Len) :-
    Len >= 4.
revolution_applies(seq, Len) :-
    Len >= 4,
    seq_revolution(on).

% =============================================
% 8. 反則上がり
% =============================================
% 最後の 1 手に「上がってはいけないカード」が含まれていたか。
% 何が禁じ手になるかは、採用しているローカルルールによって変わる。

foul_finish(Cards, Rev) :-
    foul_finish_rule(on),
    member(C, Cards),
    foul_card(C, Rev),
    !.

foul_card(card(joker, _), _).
foul_card(card(S, 2), Rev) :-
    S \== joker,
    Rev \== true.                       % 通常時の 2
foul_card(card(S, 3), Rev) :-
    S \== joker,
    Rev == true.                        % 革命時の 3
foul_card(card(_, 8), _) :-
    eight_cut(M),
    M \== off.
foul_card(card(_, j), _) :-
    jack_back(M),
    M \== off.

% 10捨て・7渡しで「手札がマイナスになる」ときの反則上がり。
% 捨てる (渡す) 枚数を手札が賄えなければ該当する。
foul_shortage(Required, Remaining) :-
    foul_finish_rule(on),
    Required > Remaining.

% =============================================
% 8.1 10捨て・7渡し
% =============================================
% 出した 10 (7) の枚数だけ、手札から要らないカードを捨てる (次の人へ渡す)。
% 階段に含まれる 10 や 7 も数える。

ten_discard_count(Cards, Count) :-
    (   ten_discard(on)
    ->  count_rank(Cards, 10, Count)
    ;   Count = 0
    ).

seven_give_count(Cards, Count) :-
    (   seven_give(on)
    ->  count_rank(Cards, 7, Count)
    ;   Count = 0
    ).

count_rank(Cards, Rank, Count) :-
    findall(C, ( member(C, Cards), C = card(S, Rank), S \== joker ), Matched),
    length(Matched, Count).

% Jバックを出した人が選べるか (任意適用の設定か)
jack_back_optional :-
    jack_back(Mode),
    Mode \== off,
    jack_back_mode(optional).

% =============================================
% 9. 最初の一手 (♦3 ルール)
% =============================================
% first_lead_ok(手札, 出すカード) … 1 ゲーム目の最初の一手として許されるか

first_lead_ok(_, Cards) :-
    first_play_rule(Rule),
    (   Rule == diamond3_must
    ->  opening_card(Open),
        Cards == [Open]
    ;   true
    ).

% =============================================
% 10. 称号・交換・得点 (daifugo.pl の title/3, exchange/4 を利用)
% =============================================

% 強さの値から数字に戻す (場に出ている手を「9 のペア」と書くため)
strength_rank(99, _, joker) :- !.
strength_rank(Strength, Rev, Rank) :-
    rank(Rank),
    rank_strength(Rank, Rev, Strength),
    !.

% titles(人数, 順位順の称号リスト)
titles(N, Titles) :-
    numlist(1, N, Ranks),
    findall(T, ( member(R, Ranks), title(N, R, T) ), Titles).

% exchange_list(人数, 上位順位, 下位順位, 枚数)
% 3 つの findall/3 は同じ順序で解を並べるので、位置どうしが対応する。
exchange_list(N, Highs, Lows, Counts) :-
    findall(H, exchange(N, H, _, _), Highs),
    findall(L, exchange(N, _, L, _), Lows),
    findall(C, exchange(N, _, _, C), Counts).

% rank_point_list(人数, 順位順の得点)
rank_point_list(N, Points) :-
    numlist(1, N, Ranks),
    findall(P, ( member(R, Ranks), rank_points(N, R, P) ), Points).
