% =============================================
% napoleon_ai.pl
%
% CPU の思考。カードを出すところは「合法手のなかから一様にランダム」。
%
% Prolog には乱数が無いので、Go から渡した Seed (非負整数) を
% 選択肢の数で割った余りで選ぶ。どれが合法かの判断は
% napoleon_rules.pl の legal_cards/4 に任せてある。
%
% ビッド (競り) と副官指名・埋め札の入れ替えは、完全な一様ランダムだと
% ゲームが成立しない (宣言が 20 枚まで際限なく吊り上がる、
% せっかく引いた ♠A を捨てる) ので、選択肢の作り方だけを絞っている。
% 絞ったあとの選択はやはりランダム。
%
% 読み込み順: compat.pl -> napoleon.pl -> napoleon_rules.pl -> napoleon_ai.pl
% =============================================

% =============================================
% 1. 出すカードを選ぶ
% =============================================

% choose_card(手札, このトリックに出ているカード, 台札 (nil ならリード),
%             切り札, Seed, 選んだカード)
%
% セイムツーで取れる見込みがあるときだけ、その 2 を選ぶ。それ以外は
% 合法手から一様にランダム。
choose_card(Hand, Trick, Lead, Trump, Seed, Card) :-
    legal_cards(Hand, Lead, Trump, Legal),
    Legal = [_|_],
    (   same_two_pick(Legal, Trick, Lead, Trump, Two)
    ->  Card = Two
    ;   pick(Legal, Seed, Card)
    ).

% same_two_pick(合法手, このトリックに出ているカード, 台札, 切り札, 出す 2)
%
% 「台札と同じマークだけが出揃い、その中に 2 があればその 2 が勝つ」ので、
% まだ誰もマークを崩していないうちに台札と同じマークの 2 を出す。
% あとの人がマークを崩せば空振りするが、2 はもともといちばん弱い札なので
% 出し損にはならない。自分が最後の 1 人なら確実に取れる。
same_two_pick(Legal, Trick, Lead, Trump, card(LeadSuit, 2)) :-
    same_two(Mode),
    Mode \== off,
    lead_suit_of(Lead, Trump, LeadSuit),
    LeadSuit \== none,
    (   Mode == not_trump           % 台札が切り札のマークなら発動しない設定
    ->  LeadSuit \== Trump
    ;   true
    ),
    all_same_suit(Trick, LeadSuit), % まだ誰もマークを崩していない
    member(card(LeadSuit, 2), Legal),
    !.

% Seed を選択肢の数で割った余りで選ぶ
pick(Choices, Seed, Choice) :-
    length(Choices, N),
    N > 0,
    Index is Seed mod N,
    nth0(Index, Choices, Choice).

% =============================================
% 2. ナポレオン宣言
% =============================================
% choose_bid(手札, 今の宣言, Seed, 選択)
%   選択は pass か bid(枚数, マーク)。
%
% 候補は「パス + 今より 1 段階上の宣言 (マーク違いを含めて 4 通りまで)」。
%
% パスは複数回ぶん入れてあり、その本数は宣言が最低枚数からどれだけ
% 吊り上がっているかで増える。一様に選ぶと宣言が 20 枚まで際限なく
% 上がってしまい、ナポレオン軍がまず勝てないゲームになるため。

choose_bid(Hand, Current, Seed, Choice) :-
    bid_candidates(Hand, Current, Candidates),
    pick(Candidates, Seed, Choice).

bid_candidates(Hand, Current, Candidates) :-
    higher_bids(Current, Higher),
    take(4, Higher, Next),
    keep_affordable(Hand, Next, Affordable),
    pass_weight(Current, Weight),
    repeat_pass(Weight, Passes),
    append(Passes, Affordable, Candidates).

% 今の宣言が最低枚数から 1 枚上がるごとに、パスを 2 つぶん厚くする。
pass_weight(nil, 3) :- !.
pass_weight(bid(Count, _), Weight) :-
    min_bid(Min),
    Over is Count - Min,
    (   Over < 0
    ->  Weight = 3
    ;   Weight is 3 + Over * 2
    ).

repeat_pass(0, []) :- !.
repeat_pass(N, [pass|Rest]) :-
    N > 0,
    N1 is N - 1,
    repeat_pass(N1, Rest).

% 手札にそのマークが 2 枚も無ければ、切り札として宣言しない。
% ここだけは「無謀すぎる宣言」を弾くための最低限の選り好み。
keep_affordable(_, [], []).
keep_affordable(Hand, [bid(Count, Suit)|Rest], Kept) :-
    (   count_suit(Hand, Suit, N),
        N >= 2
    ->  Kept = [bid(Count, Suit)|More]
    ;   Kept = More
    ),
    keep_affordable(Hand, Rest, More).

count_suit(Hand, Suit, Count) :-
    findall(C, ( member(C, Hand), C = card(Suit, _) ), Cs),
    length(Cs, Count).

% =============================================
% 3. 副官の指名
% =============================================
% choose_aide(手札, 切り札, Seed, 指名するカード)
%
% 三役 (♠A・正J・裏J) とジョーカーのうち、自分が持っていないものの
% いちばん強い 1 枚を指名する。強さの並びはジョーカーの流儀で変わるので
% power/4 に任せる。自分の手札を指名すると副官なしのゲームになるので、
% 持っているものは候補から外す。
%
% 三役もジョーカーも全部自分の手札にあるときだけ、各マークの A・K から
% ランダムに選ぶ (それも無ければ手札以外の全てから)。

choose_aide(Hand, Trump, _Seed, Card) :-
    strong_aide_candidates(Hand, Trump, [Card|_]),
    !.
choose_aide(Hand, Trump, Seed, Card) :-
    aide_candidates(Hand, Trump, Candidates),
    pick(Candidates, Seed, Card).

% 三役とジョーカーのうち手札に無いものを、強い順に並べる。
% 強い順に並べたいので、強さの符号を反転させた値を鍵にして昇順に並べる。
strong_aide_candidates(Hand, Trump, Sorted) :-
    findall(Key-Card,
            ( strong_card(Trump, Card),
              \+ member(Card, Hand),
              power(Card, Trump, Trump, Power),
              Key is -Power ),
            Pairs0),
    sort(Pairs0, Pairs),              % 重複を消して、強い順に並べる
    pairs_values(Pairs, Sorted),
    Sorted = [_|_].

strong_card(Trump, Card) :-
    mighty_role(Card, Trump).
strong_card(_, Card) :-
    uses_joker,
    joker_card(Card).

aide_candidates(Hand, Trump, Candidates) :-
    findall(C, aide_candidate(Hand, Trump, C), Cs),
    list_to_set(Cs, Candidates0),
    (   Candidates0 == []
    ->  deck(Deck),
        subtract(Deck, Hand, Candidates)     % 万一の保険
    ;   Candidates = Candidates0
    ).

aide_candidate(Hand, Trump, Card) :-
    (   mighty_role(Card, Trump)
    ;   suit(S), Card = card(S, a)
    ;   suit(S), Card = card(S, k)
    ),
    \+ member(Card, Hand).

% =============================================
% 4. 埋め札との入れ替え
% =============================================
% choose_discard(手札, 枚数, 切り札, 捨てるカード)
%
% ランダムに捨てると ♠A を伏せてしまうので、ここは「弱い順」で選ぶ。
% 絵札は得点になるので残し、切り札と三役はもちろん残す。

choose_discard(Hand, Count, Trump, Cards) :-
    findall(V-C,
            ( member(C, Hand),
              discard_value(C, Trump, V) ),
            Pairs),
    keysort(Pairs, Sorted),
    pairs_values(Sorted, Ordered),
    take(Count, Ordered, Cards).

% 小さいほど先に捨てる
discard_value(Card, Trump, 10000) :-
    mighty_role(Card, Trump),
    !.
discard_value(Card, _, 9000) :-
    is_joker(Card),
    !.
discard_value(card(Trump, Rank), Trump, Value) :-
    !,
    rank_value(Rank, V),
    Value is 5000 + V.
discard_value(Card, _, Value) :-
    Card = card(_, Rank),
    rank_value(Rank, V),
    (   point_card(Card)
    ->  Value is 1000 + V
    ;   Value = V
    ).
