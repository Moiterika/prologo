% =============================================
% napoleon_rules.pl
%
% napoleon.pl の基本ルールの上に「ローカルルール」を重ねるレイヤ。
%
% napoleon.pl は変更せず、その述語
%   base_power/4, can_play/3, strongest/4, mighty_role/2,
%   point_card/1, bid_stronger/4, game_config/2
% を呼び出すことでルールを組み立てる。
%
% 読み込み順: compat.pl -> napoleon.pl -> napoleon_rules.pl -> napoleon_ai.pl
% =============================================

% =============================================
% 1. 設定 (ゲーム開始前に選ぶローカルルール)
% =============================================

:- dynamic players/1.
:- dynamic min_bid/1.
:- dynamic same_two/1.
:- dynamic yoromeki/1.
:- dynamic joker_rule/1.
:- dynamic joker_free/1.
:- dynamic joker_lead/1.
:- dynamic joker_demand/1.
:- dynamic mighty_exempt/1.
:- dynamic scoring_rule/1.
:- dynamic slam_rule/1.

% 人数 (4 / 5 / 6)。5 人が基本。
players(5).

% 最低宣言枚数 (10 / 12 / 14)
min_bid(10).

% セイムツー
%   off       … なし
%   on        … 台札と同じマークだけが出揃い、2 があればその 2 が勝つ
%   not_trump … 上に加えて、台札が切り札のときは発動しない
same_two(on).

% よろめき
%   off    … なし
%   on     … ♠A と ♥Q が同じトリックに出たら ♥Q が勝つ
%   return … よろめき返し。さらに ♥K がいたら ♥K が勝つ
yoromeki(off).

% ジョーカーの強さ
%   none     … ジョーカーを使わない (52 枚)
%   top      … 最強 (ジョーカー > ♠A)
%   second   … 2 番手 (♠A > ジョーカー > 正J)
%   below    … 三役未満・切り札より強い
%   bottom   … 最弱
joker_rule(none).

% ジョーカーを台札に関係なくいつでも出せるか
%   off / on
joker_free(off).

% ジョーカーが台札のときの請求
%   none  … 請求なし。他のプレイヤーは何を出してもよい
%   trump … 切り札請求。切り札を持っていれば必ず出す
joker_lead(none).

% ジョーカー請求。♠3 が台札なら、ジョーカーを持っている人は必ず出す
%   off / on
joker_demand(off).

% 三役の拒否権。台札のマークでも三役なら温存してよい
%   off / on
mighty_exempt(off).

% 得点表
%   twelfth   … 『12人目のナポレオン』方式 (ナポレオン ±2、副官 ±1、連合軍 ∓1)
%   wikipedia … 宣言枚数に応じて重みを付ける方式
scoring_rule(twelfth).

% 20 枚独占したときの扱い
%   normal  … 普通に数える
%   double  … 得点が 2 倍
%   reverse … 20 枚取った側の負け。かつ得点が 2 倍
slam_rule(normal).

% --- 設定の書き換え ---

set_players(N) :-
    game_config(N, _),
    retractall(players(_)),
    assertz(players(N)).

set_min_bid(N) :-
    member(N, [10, 12, 14]),
    retractall(min_bid(_)),
    assertz(min_bid(N)).

set_same_two(M) :-
    member(M, [off, on, not_trump]),
    retractall(same_two(_)),
    assertz(same_two(M)).

set_yoromeki(M) :-
    member(M, [off, on, return]),
    retractall(yoromeki(_)),
    assertz(yoromeki(M)).

set_joker_rule(M) :-
    member(M, [none, top, second, below, bottom]),
    retractall(joker_rule(_)),
    assertz(joker_rule(M)).

set_joker_free(M) :-
    member(M, [off, on]),
    retractall(joker_free(_)),
    assertz(joker_free(M)).

set_joker_lead(M) :-
    member(M, [none, trump]),
    retractall(joker_lead(_)),
    assertz(joker_lead(M)).

set_joker_demand(M) :-
    member(M, [off, on]),
    retractall(joker_demand(_)),
    assertz(joker_demand(M)).

set_mighty_exempt(M) :-
    member(M, [off, on]),
    retractall(mighty_exempt(_)),
    assertz(mighty_exempt(M)).

set_scoring_rule(M) :-
    member(M, [twelfth, wikipedia]),
    retractall(scoring_rule(_)),
    assertz(scoring_rule(M)).

set_slam_rule(M) :-
    member(M, [normal, double, reverse]),
    retractall(slam_rule(_)),
    assertz(slam_rule(M)).

uses_joker :-
    joker_rule(M),
    M \== none.

% =============================================
% 2. デッキと配札
% =============================================

joker_card(card(joker, jk)).

deck(Deck) :-
    findall(card(S, R), ( suit(S), rank(R) ), Base),
    (   uses_joker
    ->  joker_card(J),
        append(Base, [J], Deck)
    ;   Deck = Base
    ).

hand_size(Size) :-
    players(N),
    game_config(N, Size).

% 場に伏せるカード (埋め札) の枚数
widow_size(Size) :-
    players(N),
    hand_size(Hand),
    deck(Deck),
    length(Deck, Total),
    Size is Total - N * Hand.

% =============================================
% 3. ジョーカーの強さ
% =============================================
% 三役・切り札・台札の順序のどこに挟むかをローカルルールで決める。
%   ♠A = 1000、正J = 900、裏J = 850、切り札 = 502..514、台札 = 102..114

joker_power(top, 1100).
joker_power(second, 950).
joker_power(below, 800).
joker_power(bottom, 1).

% power/4 … napoleon.pl の base_power/4 に、ジョーカーの扱いだけを足したもの。
% strongest/4 はこれを呼ぶ。
power(Card, _, _, Power) :-
    is_joker(Card),
    !,
    joker_rule(Mode),
    (   joker_power(Mode, P)
    ->  Power = P
    ;   Power = 0
    ).
power(Card, Trump, LeadSuit, Power) :-
    base_power(Card, Trump, LeadSuit, Power).

% =============================================
% 4. 台札が何を求めるか
% =============================================
% lead_requirement(台札のカード, 切り札, 要求)
%   none          … 何を出してもよい
%   suit(S)       … S を持っていれば S を出す
%   joker_or(S)   … ジョーカーを持っていればジョーカー、無ければ S でフォロー

lead_requirement(nil, _, none) :- !.
lead_requirement(Card, Trump, Req) :-
    is_joker(Card),
    !,
    joker_lead(Mode),
    (   Mode == trump
    ->  Req = suit(Trump)
    ;   Req = none
    ).
lead_requirement(card(s, 3), _, joker_or(s)) :-
    joker_demand(on),
    !.
lead_requirement(card(Suit, _), _, suit(Suit)).

% =============================================
% 5. 合法手
% =============================================
% legal_play(手札, 台札のカード (nil ならリード), 切り札, 出すカード)

legal_play(Hand, Lead, Trump, Card) :-
    member(Card, Hand),
    lead_requirement(Lead, Trump, Req),
    satisfies(Req, Hand, Trump, Card).

legal_cards(Hand, Lead, Trump, Cards) :-
    findall(C, legal_play(Hand, Lead, Trump, C), Cards0),
    list_to_set(Cards0, Cards).

satisfies(none, _, _, _).
satisfies(joker_or(Suit), Hand, Trump, Card) :-
    (   holds_joker(Hand)
    ->  is_joker(Card)
    ;   satisfies(suit(Suit), Hand, Trump, Card)
    ).
satisfies(suit(Suit), Hand, Trump, Card) :-
    (   free_card(Card)
    ->  true                                  % ジョーカーの自由出し
    ;   must_follow(Hand, Suit, Trump)
    ->  Card = card(Suit, _)
    ;   true                                  % 台札のマークが (実質) 無い
    ).

holds_joker(Hand) :-
    member(C, Hand),
    is_joker(C),
    !.

free_card(Card) :-
    is_joker(Card),
    joker_free(on).

% 台札のマークを「出さなければならない」状態か。
% 三役に拒否権がある設定では、三役しか持っていなければ従う義務はない。
must_follow(Hand, Suit, Trump) :-
    member(card(Suit, Rank), Hand),
    \+ exempt(card(Suit, Rank), Trump),
    !.

exempt(Card, Trump) :-
    mighty_exempt(on),
    mighty_role(Card, Trump).

% =============================================
% 6. トリックの勝者 (例外規定つき)
% =============================================
% trick_winner(出されたカード, 切り札, 台札のマーク, 勝ち札, 理由)
%   理由は same_two / yoromeki / yoromeki_return / normal のいずれか。

trick_winner(Cards, Trump, LeadSuit, Winner, same_two) :-
    same_two_applies(Cards, Trump, LeadSuit, Winner),
    !.
trick_winner(Cards, Trump, LeadSuit, Winner, Reason) :-
    yoromeki_applies(Cards, Trump, LeadSuit, Winner, Reason),
    !.
trick_winner(Cards, Trump, LeadSuit, Winner, normal) :-
    strongest(Cards, Trump, LeadSuit, Winner).

% --- セイムツー ---
% 台札と同じマークだけが出揃い、その中に 2 があれば、その 2 が勝つ。
% ジョーカーが混ざっていると (マークが違うので) 成立しない。
same_two_applies(Cards, Trump, LeadSuit, card(LeadSuit, 2)) :-
    same_two(Mode),
    Mode \== off,
    LeadSuit \== none,
    (   Mode == not_trump
    ->  LeadSuit \== Trump
    ;   true
    ),
    all_same_suit(Cards, LeadSuit),
    member(card(LeadSuit, 2), Cards).

all_same_suit([], _).
all_same_suit([card(S, _)|Rest], S) :-
    all_same_suit(Rest, S).

% --- よろめき ---
% ♠A と ♥Q が同じトリックに出たら ♥Q が勝つ。
% さらに ♥K がいたら ♥K が勝つ (よろめき返し)。
%
% 本来 ♠A が勝つ場面でだけ起きる例外なので、
% ♠A より強いカード (最強設定のジョーカーなど) がいれば発動しない。
yoromeki_applies(Cards, Trump, LeadSuit, Winner, Reason) :-
    yoromeki(Mode),
    Mode \== off,
    member(card(s, a), Cards),
    member(card(h, q), Cards),
    strongest(Cards, Trump, LeadSuit, card(s, a)),
    (   Mode == return,
        member(card(h, k), Cards)
    ->  Winner = card(h, k),
        Reason = yoromeki_return
    ;   Winner = card(h, q),
        Reason = yoromeki
    ).

% =============================================
% 7. ナポレオン宣言 (ビッド)
% =============================================

% valid_bid(枚数, マーク) … 宣言として成立する範囲か
valid_bid(Count, Suit) :-
    min_bid(Min),
    integer(Count),
    Count >= Min,
    Count =< 20,
    suit(Suit).

% bid_ok(枚数, マーク, 今の宣言)
%   今の宣言は nil (まだ誰も宣言していない) か bid(枚数, マーク)。
bid_ok(Count, Suit, nil) :-
    !,
    valid_bid(Count, Suit).
bid_ok(Count, Suit, bid(CurCount, CurSuit)) :-
    valid_bid(Count, Suit),
    bid_stronger(Count, Suit, CurCount, CurSuit).

% 今の宣言を上回る宣言を、弱い順に列挙する
higher_bids(Current, Bids) :-
    min_bid(Min),
    findall(bid(Count, Suit),
            ( between(Min, 20, Count),
              suit(Suit),
              bid_ok(Count, Suit, Current) ),
            Unsorted),
    sort_bids(Unsorted, Bids).

sort_bids(Bids, Sorted) :-
    findall(V-B,
            ( member(B, Bids),
              B = bid(C, S),
              bid_value(C, S, V) ),
            Pairs),
    keysort(Pairs, Ordered),
    pairs_values(Ordered, Sorted).

% =============================================
% 8. 精算
% =============================================

% round_result(宣言枚数, ナポレオン軍が取った絵札, 勝者, 倍率)
round_result(Bid, Taken, Outcome, Multiplier) :-
    total_point_cards(Total),
    slam_rule(Mode),
    (   Taken =:= Total
    ->  slam_outcome(Mode, napoleon, Outcome, Multiplier)
    ;   Taken =:= 0
    ->  slam_outcome(Mode, allied, Outcome, Multiplier)
    ;   (   Taken >= Bid
        ->  Outcome = napoleon
        ;   Outcome = allied
        ),
        Multiplier = 1
    ).

slam_outcome(normal, Side, Side, 1).
slam_outcome(double, Side, Side, 2).
slam_outcome(reverse, napoleon, allied, 2).
slam_outcome(reverse, allied, napoleon, 2).

% base_points(宣言枚数, 副官なしか, ナポレオン, 副官, 連合軍 1 人あたり)
% 勝った側が得る絶対値。負けた側は同じ値を失う (ゼロサム)。
base_points(Bid, Solo, Napoleon, Aide, Allied) :-
    scoring_rule(Rule),
    base_points_(Rule, Bid, Solo, Napoleon, Aide, Allied).

base_points_(twelfth, _, Solo, Napoleon, Aide, 1) :-
    (   Solo == true
    ->  Napoleon = 4, Aide = 0
    ;   Napoleon = 2, Aide = 1
    ).
base_points_(wikipedia, Bid, Solo, Napoleon, Aide, Allied) :-
    min_bid(Min),
    Allied is Bid - (Min - 1),
    (   Solo == true
    ->  Napoleon is Allied * 4, Aide = 0
    ;   Napoleon is Allied * 2, Aide = Allied
    ).

% round_scores(宣言枚数, 取った絵札, 副官なしか,
%              勝者, ナポレオンの点, 副官の点, 連合軍 1 人あたりの点)
% 点は符号つき (プラスが獲得)。
round_scores(Bid, Taken, Solo, Outcome, NapPoints, AidePoints, AlliedPoints) :-
    round_result(Bid, Taken, Outcome, Multiplier),
    base_points(Bid, Solo, Nap, Aide, Allied),
    (   Outcome == napoleon
    ->  NapPoints is Nap * Multiplier,
        AidePoints is Aide * Multiplier,
        AlliedPoints is -Allied * Multiplier
    ;   NapPoints is -Nap * Multiplier,
        AidePoints is -Aide * Multiplier,
        AlliedPoints is Allied * Multiplier
    ).

% =============================================
% 9. 台札のマーク
% =============================================
% ジョーカーがリードのときは、切り札請求があるかどうかで変わる。
% 請求が無ければ「マーク無し (none)」になり、誰も台札には縛られない。

lead_suit_of(nil, _, none) :- !.
lead_suit_of(Card, Trump, Suit) :-
    is_joker(Card),
    !,
    joker_lead(Mode),
    (   Mode == trump
    ->  Suit = Trump
    ;   Suit = none
    ).
lead_suit_of(card(Suit, _), _, Suit).
