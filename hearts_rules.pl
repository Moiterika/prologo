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

% 出されたカードの中にハートが含まれるか
contains_heart(Cards) :-
    member(card(h, _), Cards),
    !.

% 手札にハート以外のカードがあるか
has_non_heart(Hand) :-
    member(card(S, _), Hand),
    S \== h,
    !.

% 第1トリックで出せる「失点なしカード」を持っているか
has_safe_card(Hand, Lead) :-
    can_play(Hand, Lead, C),
    \+ point_card(C),
    !.

% =============================================
% 5. 合法手 (hearts.pl の can_play/3 に追加制約を重ねる)
% =============================================
% legal_play(手札, 台札, トリック番号, ハートブレイク(true/false), 出すカード)

% (a) 第1トリックのリード: 必ず開始カード(2♣)を出す
legal_play(Hand, nil, 1, _, Card) :-
    !,
    opening_card(Card),
    member(Card, Hand).

% (b) 第2トリック以降のリード: ハート未ブレイクならハートでリードできない
%     (手札がハートのみの場合は例外)
legal_play(Hand, nil, _, Broken, Card) :-
    !,
    (   ( Broken == true ; \+ has_non_heart(Hand) )
    ->  can_play(Hand, nil, Card)
    ;   can_play(Hand, nil, Card),
        Card = card(Suit, _),
        Suit \== h
    ).

% (c) 第1トリックのフォロー: 失点カードは出せない
%     (失点カードしか出せない場合は例外)
legal_play(Hand, Lead, 1, _, Card) :-
    !,
    (   has_safe_card(Hand, Lead)
    ->  can_play(Hand, Lead, Card),
        \+ point_card(Card)
    ;   can_play(Hand, Lead, Card)
    ).

% (d) 通常のフォロー: hearts.pl のマストフォロー規則そのまま
legal_play(Hand, Lead, _, _, Card) :-
    can_play(Hand, Lead, Card).

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

index_of(X, List, Index) :-
    index_of_(List, X, 0, Index).

index_of_([X|_], Y, I, I) :-
    X == Y,
    !.
index_of_([_|T], Y, I0, I) :-
    I1 is I0 + 1,
    index_of_(T, Y, I1, I).

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
