% =============================================
% hearts_ai.pl
%
% CPU の思考ルーチン。
% hearts.pl の rank_value/2, is_stronger/2, is_suit/2,
% determine_winner/3, card_point/2 をそのまま利用して、
% 「どのカードを出すか」「何を渡すか」だけを決める。
%
% 読み込み順:
%   compat.pl -> hearts.pl -> hearts_rules.pl -> hearts_ai.pl
% =============================================

% =============================================
% 1. ソート補助 (hearts.pl の rank_value/2 を利用)
% =============================================

% ランクの弱い順に並べる
by_rank(Cards, Sorted) :-
    findall(V-C,
            ( member(C, Cards),
              C = card(_, R),
              rank_value(R, V) ),
            Pairs),
    keysort(Pairs, SortedPairs),
    pairs_values(SortedPairs, Sorted).

pairs_values([], []).
pairs_values([_-V|T], [V|Vs]) :-
    pairs_values(T, Vs).

lowest(Cards, Low) :-
    by_rank(Cards, [Low|_]).

highest(Cards, High) :-
    by_rank(Cards, Sorted),
    last_(Sorted, High).

last_([X], X) :- !.
last_([_|T], X) :- last_(T, X).

% =============================================
% 2. 危険なカードの判定
% =============================================

% Q♠ を引き取らされる恐れのある高位スペード (Q, K, A)
risky_spade(card(s, R)) :-
    rank_value(R, V),
    V >= 12.

% 早めに捨てたい K♠ / A♠
high_spade(card(s, R)) :-
    rank_value(R, V),
    V >= 13.

% =============================================
% 3. 出すカードの決定
% =============================================
% choose_card(手札, 台札, 場に出ているカード, トリック番号,
%             ハートブレイク, 選んだカード)

choose_card(Hand, Lead, Trick, TrickNo, Broken, Card) :-
    legal_cards(Hand, Lead, TrickNo, Broken, Legal),
    Legal = [_|_],
    choose_from(Lead, Trick, Legal, Card),
    !.

% --- リードのとき ---
% 高位スペードを避けつつ、できるだけ弱いカードでリードする
choose_from(nil, _, Legal, Card) :-
    !,
    (   exclude(risky_spade, Legal, Safe),
        Safe = [_|_]
    ->  lowest(Safe, Card)
    ;   lowest(Legal, Card)
    ).

% --- フォローのとき ---
choose_from(card(LeadSuit, _), Trick, Legal, Card) :-
    (   include(is_suit(LeadSuit), Legal, Follow),
        Follow = [_|_]
    ->  follow_pick(LeadSuit, Trick, Follow, Card)
    ;   discard_pick(Legal, Card)
    ).

% 台札スートを持っている場合:
%   現在の勝ちカードより下で「一番高いカード」を出す (高いカードを安全に処理)
%   下をくぐれないなら「一番低いカード」を出す (取らされる被害を最小化)
follow_pick(LeadSuit, Trick, Follow, Card) :-
    determine_winner(Trick, LeadSuit, Winner),
    (   include(weaker_than(Winner), Follow, Under),
        Under = [_|_]
    ->  highest(Under, Card)
    ;   lowest(Follow, Card)
    ).

weaker_than(Winner, Card) :-
    is_stronger(Winner, Card).

% 台札スートを持っていない場合 (スートを切る):
%   Q♠ > K♠/A♠ > 高いハート > 一番高いカード の順で捨てる
discard_pick(Legal, Card) :-
    (   member(card(s, q), Legal)
    ->  Card = card(s, q)
    ;   include(high_spade, Legal, HighSpades),
        HighSpades = [_|_]
    ->  highest(HighSpades, Card)
    ;   include(is_suit(h), Legal, Hearts),
        Hearts = [_|_]
    ->  highest(Hearts, Card)
    ;   highest(Legal, Card)
    ).

% =============================================
% 4. パスするカードの決定
% =============================================
% 危険度の高い順に Count 枚選ぶ

choose_pass(Hand, Count, Cards) :-
    findall(D-C,
            ( member(C, Hand),
              pass_danger(C, D0),
              D is -D0 ),
            Pairs),
    keysort(Pairs, Sorted),
    pairs_values(Sorted, Ordered),
    take(Count, Ordered, Cards).

pass_danger(card(s, q), 100) :- !.
pass_danger(card(s, a), 90) :- !.
pass_danger(card(s, k), 85) :- !.
pass_danger(card(h, R), D) :-
    !,
    rank_value(R, V),
    D is V + 20.
pass_danger(card(_, R), D) :-
    rank_value(R, V),
    D is V.

take(0, _, []) :- !.
take(_, [], []) :- !.
take(N, [X|Xs], [X|Ys]) :-
    N > 0,
    N1 is N - 1,
    take(N1, Xs, Ys).
