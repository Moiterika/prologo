% =============================================
% daifugo.pl
%
% 大富豪の「基本ルール」。ローカルルールを一切含まない層。
%
%   - カードの定義と強さ (革命による反転を含む)
%   - 出し方 (役) の判定: 単騎 / ペア / スリーカード / フォーカード / 階段
%   - 場のカードに勝てるかの判定
%   - 上がり順から称号を決める
%
% ローカルルールの切り替えは daifugo_rules.pl が受け持つ。
% 読み込み順: compat.pl -> daifugo.pl -> daifugo_rules.pl -> daifugo_ai.pl
% =============================================

% =============================================
% 1. 基本定義 (Facts & Constants)
% =============================================

% スートの定義。マークによる強弱の差は無い (縛りの判定にだけ使う)。
suit(s).
suit(h).
suit(d).
suit(c).

% ランクの定義。大富豪では 3 が最弱で 2 が最強。
rank(3). rank(4). rank(5). rank(6). rank(7). rank(8). rank(9).
rank(10). rank(j). rank(q). rank(k). rank(a). rank(2).

% ジョーカーは card(joker, jk1) / card(joker, jk2) で表す。
% ランクを jk1/jk2 という専用のアトムにしてあるのは、card(_, 2) のような
% パターンがうっかりジョーカーに当たらないようにするため。
joker_rank(jk1).
joker_rank(jk2).

is_joker(card(joker, _)).

% =============================================
% 2. カードの強さ (Card Strength)
% =============================================
% 3 を 1、2 を 13 とする通し番号。階段の「連続」もこの番号で判定する。

base_strength(3, 1).
base_strength(4, 2).
base_strength(5, 3).
base_strength(6, 4).
base_strength(7, 5).
base_strength(8, 6).
base_strength(9, 7).
base_strength(10, 8).
base_strength(j, 9).
base_strength(q, 10).
base_strength(k, 11).
base_strength(a, 12).
base_strength(2, 13).

% 革命中は 1..13 を 13..1 に反転する (ジョーカーだけは影響を受けない)。
% Rev は true / false。
rank_strength(Rank, Rev, Value) :-
    base_strength(Rank, Base),
    (   Rev == true
    ->  Value is 14 - Base
    ;   Value = Base
    ).

% ジョーカーは常に 99。革命でも最強のまま。
card_strength(card(joker, _), _, 99) :- !.
card_strength(card(_, Rank), Rev, Value) :-
    rank_strength(Rank, Rev, Value).

% =============================================
% 3. 出し方 (役) の判定
% =============================================
% classify(出すカード, 革命中か, 種別, 枚数, 強さ, 使ったスート, ジョーカー枚数)
%
%   種別 group … 同じ数字の N 枚 (N=1 なら単騎)
%   種別 seq   … 同じマークで連続する N 枚 (N>=3、階段)
%
% ジョーカーはワイルドカードとして任意の 1 枚として扱う。
% 階段でジョーカーがどの位置を埋めたかは一意に決まらないことがあるので、
% classify/7 は「ありうる解釈」を全て列挙する非決定的な述語になっている。
% どの解釈を採るかは daifugo_rules.pl が決める。

% --- ジョーカーとそれ以外に分ける ---
split_jokers([], [], []).
split_jokers([C|Cs], Jokers, Reals) :-
    (   is_joker(C)
    ->  Jokers = [C|J1], Reals = R1
    ;   Jokers = J1, Reals = [C|R1]
    ),
    split_jokers(Cs, J1, R1).

% --- カード列から使っているスートを重複なく取り出す (ジョーカーは含まない) ---
real_suits(Cards, Suits) :-
    findall(S, ( member(card(S, _), Cards), S \== joker ), Ss),
    list_to_set(Ss, Suits).

% --- 同じ数字か ---
all_same_rank([], _).
all_same_rank([card(_, R)|Rest], R) :-
    all_same_rank(Rest, R).

% --- 同じマークか ---
all_same_suit([], _).
all_same_suit([card(S, _)|Rest], S) :-
    all_same_suit(Rest, S).

% --- 同じ数字の N 枚 ---
classify(Cards, Rev, group, N, Strength, Suits, JokerN) :-
    length(Cards, N),
    N >= 1,
    split_jokers(Cards, Jokers, Reals),
    length(Jokers, JokerN),
    real_suits(Reals, Suits),
    length(Suits, SuitCount),
    length(Reals, RealCount),
    SuitCount =:= RealCount,          % 同じカードを二重に出していない
    (   Reals == []
    ->  Strength = 99                 % ジョーカーだけで出した
    ;   Reals = [card(_, R)|_],
        all_same_rank(Reals, R),
        rank_strength(R, Rev, Strength)
    ).

% --- 同じマークで連続する N 枚 (階段) ---
classify(Cards, Rev, seq, N, Strength, [Suit], JokerN) :-
    length(Cards, N),
    N >= 3,
    split_jokers(Cards, Jokers, Reals),
    length(Jokers, JokerN),
    Reals = [card(Suit, _)|_],
    all_same_suit(Reals, Suit),
    card_positions(Reals, Positions),
    sort(Positions, Sorted),
    length(Positions, PosCount),
    length(Sorted, PosCount),         % 同じカードを二重に出していない
    seq_window(Sorted, N, Low, High),
    window_strength(Low, High, Rev, Strength).

card_positions([], []).
card_positions([card(_, R)|Rest], [P|Ps]) :-
    base_strength(R, P),
    card_positions(Rest, Ps).

% 手札の実カードを全て収める、長さ N の連続した窓 [Low, High] を列挙する。
% 足りない位置はジョーカーが埋めることになるので、窓の取り方が複数あれば
% そのぶんだけ解釈が生まれる。
seq_window(Positions, N, Low, High) :-
    min_list(Positions, Min),
    max_list(Positions, Max),
    Span is Max - Min + 1,
    Span =< N,
    LowFloor is Max - N + 1,
    (   LowFloor < 1
    ->  LowMin = 1
    ;   LowMin = LowFloor
    ),
    LowCeil is 14 - N,
    (   Min > LowCeil
    ->  LowMax = LowCeil
    ;   LowMax = Min
    ),
    LowMin =< LowMax,
    between(LowMin, LowMax, Low),
    High is Low + N - 1.

% 階段の強さは「窓のうち一番強いカード」。革命中は上下が入れ替わる。
window_strength(Low, High, Rev, Strength) :-
    (   Rev == true
    ->  Strength is 14 - Low
    ;   Strength = High
    ).

% =============================================
% 4. 場のカードに勝てるか (基本ルール)
% =============================================
% 場に出ているのと「同じ種別・同じ枚数」で、より強いもの。
% スペ3返しのような例外は daifugo_rules.pl が重ねる。

% 種別と枚数は頭部の単一化で一致を要求している。
beats(Kind, Len, Strength, field_form(Kind, Len, FieldStrength)) :-
    Strength > FieldStrength.

% =============================================
% 5. 上がり順と称号
% =============================================
% 人数によって称号の並びが変わる。真ん中は全て平民。

% title(人数, 順位(1始まり), 称号)
title(_, 1, daifugo).
title(N, N, daihinmin) :- N >= 2.
title(N, 2, fugo) :- N >= 4.
title(N, R, hinmin) :- N >= 4, R =:= N - 1.
title(N, R, heimin) :-
    R > 1,
    R < N,
    \+ ( N >= 4, R =:= 2 ),
    \+ ( N >= 4, R =:= N - 1 ).

% rank_points(人数, 順位, 順位点)  … 1位が最大、最下位が 0
rank_points(N, Rank, Points) :-
    Points is N - Rank.

% =============================================
% 6. カード交換の組み合わせ
% =============================================
% exchange(人数, 上位の順位, 下位の順位, 枚数)
% 上位は好きなカードを、下位は強いカードを渡す。

exchange(N, 1, N, 2) :- N >= 2.
exchange(N, 2, Low, 1) :- N >= 4, Low is N - 1.

% =============================================
% 7. 配札
% =============================================
% 人数で割り切れないときは、余りを先頭のプレイヤーから 1 枚ずつ配る。
% deal_sizes(人数, 総枚数, 各自の枚数)

deal_sizes(Players, Total, Sizes) :-
    Base is Total // Players,
    Extra is Total mod Players,
    numlist(1, Players, Seats),
    findall(Size,
            ( member(I, Seats),
              (   I =< Extra
              ->  Size is Base + 1
              ;   Size = Base
              ) ),
            Sizes).

% 配られた状態が正当かどうかの検証 (手続きとしての「配る処理」ではない)
validate_deal(Hands, Deck) :-
    length(Deck, Total),
    length(Hands, Players),
    deal_sizes(Players, Total, Sizes),
    hand_sizes(Hands, Sizes),
    flatten(Hands, All),
    length(All, Total),
    is_set(All),
    subset(All, Deck).

hand_sizes([], []).
hand_sizes([H|Hs], [S|Ss]) :-
    length(H, S),
    hand_sizes(Hs, Ss).
