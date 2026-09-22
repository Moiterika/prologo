% =============================================
% hearts_rules.pl
%
% hearts.pl の基本ルール(フォロー義務・トリック勝者・カード点数)の
% 上に、実戦に必要なハーツの追加ルールを重ねたレイヤ。
%
% 方針: hearts.pl は一切変更しない。
%       ここでは hearts.pl の述語
%         suit/1, rank/1, excluded_cards/1, can_play/3,
%         card_point/2, calculate_score/2, determine_winner/3
%       を「呼び出す」ことでルールを組み立てる。
%
% 読み込み順: compat.pl -> hearts.pl -> hearts_rules.pl
% =============================================

% =============================================
% 1. デッキ (hearts.pl の suit/1, rank/1, excluded_cards/1 から生成)
% =============================================

deck(Deck) :-
    excluded_cards(Excluded),
    findall(card(S, R),
            ( suit(S),
              rank(R),
              \+ member(card(S, R), Excluded) ),
            Deck).

% =============================================
% 2. 得点まわり (hearts.pl の card_point/2 を利用)
% =============================================

% 失点を持つカードか (ハート全部 と スペードのQ)
point_card(Card) :-
    card_point(Card, P),
    P > 0.

% 1ラウンドに存在する失点の総和 (通常 26点)
total_round_points(Total) :-
    deck(Deck),
    findall(P, ( member(C, Deck), card_point(C, P) ), Points),
    sum_list(Points, Total).

% =============================================
% 3. 第1トリックの開始カード
% =============================================
% 通常は クラブの2。3人/5人戦では 2♣ が除外されるので クラブの3。

opening_card(card(c, Rank)) :-
    excluded_cards(Excluded),
    (   member(card(c, 2), Excluded)
    ->  Rank = 3
    ;   Rank = 2
    ).

% =============================================
% 4. ハートブレイク判定の補助
% =============================================

% 出されたカードの中に得点札 (ハート全般と ♠Q) が含まれるか。
% これが出た時点で「ブレイク」し、以降ハートでリードできるようになる。
contains_point_card(Cards) :-
    member(C, Cards),
    point_card(C),
    !.

% 手札にハート以外のカードがあるか
has_non_heart(Hand) :-
    member(card(S, _), Hand),
    S \== h,
    !.

% =============================================
% 4.1 第1トリックの流儀 (ゲーム開始時に選ぶ)
% =============================================

:- dynamic first_trick_points/1.
:- dynamic first_trick_lead/1.
:- dynamic hearts_break_rule/1.

% first_trick_points(Mode)
%   forbid_all    … 点数札を出せない (ハート全般と ♠Q)  ※ no blood on the first trick
%   forbid_hearts … ハートだけ出せない (♠Q は出せる)
%   allow         … 制限なし  ※既定
first_trick_points(allow).

% first_trick_lead(Mode)
%   opening_card … 第1トリックは開始カード(♣2)から始める
%   free         … ♣2 の持ち主が先手だが、出すカードは自由
first_trick_lead(opening_card).

set_first_trick_points(Mode) :-
    member(Mode, [forbid_all, forbid_hearts, allow]),
    retractall(first_trick_points(_)),
    assertz(first_trick_points(Mode)).

set_first_trick_lead(Mode) :-
    member(Mode, [opening_card, free]),
    retractall(first_trick_lead(_)),
    assertz(first_trick_lead(Mode)).

% hearts_break_rule(Mode)
%   on  … 得点札 (ハート全般と ♠Q) が場に出るまでハートでリードできない
%   off … いつでもハートでリードできる  ※既定
hearts_break_rule(off).

set_hearts_break_rule(Mode) :-
    member(Mode, [on, off]),
    retractall(hearts_break_rule(_)),
    assertz(hearts_break_rule(Mode)).

% 第1トリックで出すのを禁じられているカードか
first_trick_banned(Card) :-
    first_trick_points(Mode),
    banned_on_first_trick(Mode, Card).

banned_on_first_trick(forbid_all, Card) :-
    point_card(Card).
banned_on_first_trick(forbid_hearts, card(h, _)).
% allow のときは該当する節が無いので、何も禁じられない。

% =============================================
% 5. 合法手 (hearts.pl の can_play/3 に追加制約を重ねる)
% =============================================
% legal_play(手札, 台札, トリック番号, ハートブレイク(true/false), 出すカード)

% --- 基本の候補 ---
% リードのときは (ハートブレイク規則が有効なら)「ブレイク前はハートで
% リードできない」を重ね、フォローのときは hearts.pl のマストフォローを
% そのまま使う。
base_play(Hand, nil, Broken, Card) :-
    !,
    (   heart_lead_allowed(Hand, Broken)
    ->  can_play(Hand, nil, Card)
    ;   can_play(Hand, nil, Card),
        Card = card(Suit, _),
        Suit \== h
    ).
base_play(Hand, Lead, _, Card) :-
    can_play(Hand, Lead, Card).

% ハートでリードしてよいか。
% 規則を使わない設定、既に得点札が出ている、手札がハートだけ、のいずれか。
heart_lead_allowed(_, _) :-
    hearts_break_rule(off),
    !.
heart_lead_allowed(_, Broken) :-
    Broken == true,
    !.
heart_lead_allowed(Hand, _) :-
    \+ has_non_heart(Hand).

% --- 第1トリック ---
% 禁じられていないカードを持っているなら、それしか出せない。
% 禁じられたカードしか無い場合は出さざるを得ないので許す。
first_trick_play(Hand, nil, _, Card) :-
    first_trick_lead(opening_card),
    !,
    opening_card(Card),
    member(Card, Hand).
first_trick_play(Hand, Lead, Broken, Card) :-
    (   has_unbanned_card(Hand, Lead, Broken)
    ->  base_play(Hand, Lead, Broken, Card),
        \+ first_trick_banned(Card)
    ;   base_play(Hand, Lead, Broken, Card)
    ).

has_unbanned_card(Hand, Lead, Broken) :-
    base_play(Hand, Lead, Broken, C),
    \+ first_trick_banned(C),
    !.

% --- まとめ ---
legal_play(Hand, Lead, TrickNo, Broken, Card) :-
    (   TrickNo =:= 1
    ->  first_trick_play(Hand, Lead, Broken, Card)
    ;   base_play(Hand, Lead, Broken, Card)
    ).

% 合法手の一覧
legal_cards(Hand, Lead, TrickNo, Broken, Cards) :-
    findall(C, legal_play(Hand, Lead, TrickNo, Broken, C), Cards).

% =============================================
% 6. カード交換 (パス)
% =============================================

% 交換する枚数
pass_count(3).

% ラウンド番号(1始まり)からパス方向を決める。
% 「向かい」は人数が偶数のときしか定義できないため、
% 奇数人数では 左 -> 右 -> 交換なし の3周期にする。
pass_direction(Round, Direction) :-
    total_players(N),
    (   0 is N mod 2
    ->  M is Round mod 4,
        pass_dir_of(M, Direction)
    ;   M is Round mod 3,
        pass_dir_of_odd(M, Direction)
    ).

pass_dir_of(1, left).    % 左隣へ
pass_dir_of(2, right).   % 右隣へ
pass_dir_of(3, across).  % 向かいへ
pass_dir_of(0, hold).    % 交換なし

pass_dir_of_odd(1, left).
pass_dir_of_odd(2, right).
pass_dir_of_odd(0, hold).

% pass_target(渡す人, 方向, 人数, 受け取る人)  ※インデックスは0始まり
pass_target(From, left, N, To) :-
    To is (From + 1) mod N.
pass_target(From, right, N, To) :-
    To is (From - 1 + N) mod N.
pass_target(From, across, N, To) :-
    Half is N // 2,
    To is (From + Half) mod N.
pass_target(From, hold, _, From).

% =============================================
% 7. ラウンド精算 (hearts.pl の calculate_score/2 を利用)
% =============================================

% 各プレイヤーが取ったカード束から素点を求める
raw_scores([], []).
raw_scores([Cards|Rest], [S|Ss]) :-
    calculate_score(Cards, S),
    raw_scores(Rest, Ss).

% スラム: 1人で全失点を取ると、その人が0点、他全員が満点失点
% apply_slam_rule(素点リスト, 確定点リスト, 達成者)
%   達成者は 0始まりのプレイヤー番号。達成者がいない場合は -1。
apply_slam_rule(Raw, Final, Slammer) :-
    total_round_points(Total),
    (   Total > 0,
        index_of(Total, Raw, I)
    ->  Slammer = I,
        slam_final(Raw, I, 0, Total, Final)
    ;   Slammer = -1,
        Final = Raw
    ).

% index_of/3 (0始まりの位置) は compat.pl にある。

slam_final([], _, _, _, []).
slam_final([_|T], I, Idx, Total, [S|Rest]) :-
    (   Idx =:= I
    ->  S = 0
    ;   S = Total
    ),
    Idx1 is Idx + 1,
    slam_final(T, I, Idx1, Total, Rest).

% ラウンド精算のまとめ
% round_scores(取ったカード束のリスト, 確定点リスト, スラム達成者)
round_scores(TakenLists, Final, Slammer) :-
    raw_scores(TakenLists, Raw),
    apply_slam_rule(Raw, Final, Slammer).

% =============================================
% 8. ゲーム終了判定
% =============================================

% 誰かが上限点に達したらゲーム終了
game_over(Scores, Limit) :-
    member(S, Scores),
    S >= Limit,
    !.
