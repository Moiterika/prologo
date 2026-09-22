% =============================================
% napoleon.pl
%
% ナポレオンの「基本ルール」。ローカルルールを一切含まない層。
%
%   - カードの定義
%   - 切り札・正ジャック・裏ジャック・オールマイティを含む強さの順序
%   - マストフォロー
%   - トリックの勝者判定 (例外規定を除いた素の順序)
%   - 絵札 (得点カード) の数え方
%   - ナポレオン宣言 (ビッド) の強弱
%
% ローカルルール (セイムツー・よろめき・ジョーカーの強さ …) は
% napoleon_rules.pl が重ねる。
%
% 読み込み順: compat.pl -> napoleon.pl -> napoleon_rules.pl -> napoleon_ai.pl
% =============================================

% =============================================
% 1. 基本定義 (Facts & Constants)
% =============================================

suit(s).
suit(h).
suit(d).
suit(c).

rank(2). rank(3). rank(4). rank(5). rank(6). rank(7). rank(8). rank(9).
rank(10). rank(j). rank(q). rank(k). rank(a).

% ジョーカーは 1 枚だけ。大富豪と同じく専用のランクを使う。
is_joker(card(joker, _)).

% マークの色。裏ジャック (切り札と同じ色のもう一方の J) を決めるのに使う。
color(s, black).
color(c, black).
color(h, red).
color(d, red).

% 同じ色のもう一方のマーク
same_color_partner(Suit, Partner) :-
    color(Suit, Color),
    color(Partner, Color),
    Partner \== Suit.

% =============================================
% 2. 得点カード (絵札)
% =============================================
% 10 を含めた絵札 A・K・Q・J・10 が得点。全部で 20 枚。

point_rank(a).
point_rank(k).
point_rank(q).
point_rank(j).
point_rank(10).

point_card(card(Suit, Rank)) :-
    Suit \== joker,
    point_rank(Rank).

count_points(Cards, Count) :-
    findall(C, ( member(C, Cards), point_card(C) ), Points),
    length(Points, Count).

% 1 ゲームに存在する得点カードの総数 (20)
total_point_cards(20).

% =============================================
% 3. 三役 (オールマイティ・正ジャック・裏ジャック)
% =============================================

% オールマイティ
almighty(card(s, a)).

% 正ジャック … 切り札と同じマークの J
main_jack(Trump, card(Trump, j)).

% 裏ジャック … 切り札と同じ色のもう一方のマークの J
counter_jack(Trump, card(Partner, j)) :-
    same_color_partner(Trump, Partner).

% mighty_role(カード, 切り札) … 三役のどれかか
mighty_role(Card, _) :-
    almighty(Card).
mighty_role(Card, Trump) :-
    main_jack(Trump, Card).
mighty_role(Card, Trump) :-
    counter_jack(Trump, Card).

% =============================================
% 4. カードの強さ
% =============================================
% 強い順に
%   ♠A > 正J > 裏J > 切り札の A..2 > 台札の A..2 > それ以外 (必ず負ける)
%
% ジョーカーの強さはローカルルールで変わるので、ここでは決めない。
% napoleon_rules.pl の joker_power/2 が値を与える。

rank_value(2, 2).
rank_value(3, 3).
rank_value(4, 4).
rank_value(5, 5).
rank_value(6, 6).
rank_value(7, 7).
rank_value(8, 8).
rank_value(9, 9).
rank_value(10, 10).
rank_value(j, 11).
rank_value(q, 12).
rank_value(k, 13).
rank_value(a, 14).

% 基本の強さ。三役・切り札・台札の区別だけを見る。
% LeadSuit は台札のマーク。台札が無い (ジョーカーがリードなど) 場合は none。
base_power(Card, _, _, 1000) :-
    almighty(Card),
    !.
base_power(Card, Trump, _, 900) :-
    main_jack(Trump, Card),
    !.
base_power(Card, Trump, _, 850) :-
    counter_jack(Trump, Card),
    !.
base_power(card(Trump, Rank), Trump, _, Power) :-
    !,
    rank_value(Rank, V),
    Power is 500 + V.
base_power(card(LeadSuit, Rank), _, LeadSuit, Power) :-
    !,
    rank_value(Rank, V),
    Power is 100 + V.
base_power(_, _, _, 0).

% =============================================
% 5. マストフォロー
% =============================================
% 台札と同じマークを持っていたら必ず出す。
% 三役の拒否権やジョーカーの自由出しは napoleon_rules.pl が重ねる。

has_suit([card(S, _)|_], S) :- !.
has_suit([_|Rest], S) :- has_suit(Rest, S).

% can_play(手札, 台札のマーク, 出すカード)
can_play(Hand, none, Card) :-
    member(Card, Hand).
can_play(Hand, LeadSuit, card(LeadSuit, Rank)) :-
    LeadSuit \== none,
    member(card(LeadSuit, Rank), Hand),
    has_suit(Hand, LeadSuit).
can_play(Hand, LeadSuit, Card) :-
    LeadSuit \== none,
    member(Card, Hand),
    \+ has_suit(Hand, LeadSuit).

% =============================================
% 6. トリックの勝者 (素の順序)
% =============================================
% 例外規定 (セイムツー・よろめき) は napoleon_rules.pl が先に判定し、
% どれも当てはまらないときにここへ来る。

strongest([Card], _, _, Card) :- !.
strongest([C1, C2|Rest], Trump, LeadSuit, Winner) :-
    power(C1, Trump, LeadSuit, P1),
    power(C2, Trump, LeadSuit, P2),
    (   P1 >= P2
    ->  strongest([C1|Rest], Trump, LeadSuit, Winner)
    ;   strongest([C2|Rest], Trump, LeadSuit, Winner)
    ).

% =============================================
% 7. ナポレオン宣言 (ビッド)
% =============================================
% 「切り札マークと、獲得を目指す絵札の枚数」を宣言する。
% 枚数が多いほど強く、同じ枚数なら ♠ > ♥ > ♦ > ♣ の順に強い。

bid_suit_order(c, 0).
bid_suit_order(d, 1).
bid_suit_order(h, 2).
bid_suit_order(s, 3).

bid_value(Count, Suit, Value) :-
    bid_suit_order(Suit, S),
    Value is Count * 10 + S.

% 宣言 A が宣言 B より強いか
bid_stronger(CountA, SuitA, CountB, SuitB) :-
    bid_value(CountA, SuitA, A),
    bid_value(CountB, SuitB, B),
    A > B.

% =============================================
% 8. 人数別設定
% =============================================
% game_config(人数, 1 人あたりの手札枚数)
% 場に伏せるカード (埋め札) の枚数は、残りとして決まる。

game_config(4, 12).
game_config(5, 10).
game_config(6, 8).

% 配られた状態が正当かどうかの検証
validate_deal(Hands, Widow, Deck) :-
    length(Deck, Total),
    length(Hands, Players),
    game_config(Players, HandSize),
    hand_sizes(Hands, HandSize),
    flatten(Hands, Dealt),
    append(Dealt, Widow, All),
    length(All, Total),
    is_set(All),
    subset(All, Deck).

hand_sizes([], _).
hand_sizes([H|Hs], Size) :-
    length(H, Size),
    hand_sizes(Hs, Size).
