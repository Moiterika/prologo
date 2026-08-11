% =============================================
% 1. 基本定義 (Facts & Constants)
% =============================================

% スートの定義
suit(h).
suit(d).
suit(c).
suit(s).

% ランクの定義
rank(2). rank(3). rank(4). rank(5). rank(6). rank(7). rank(8). rank(9).
rank(10). rank(j). rank(q). rank(k). rank(a).

% =============================================
% 1.1 人数別設定 (Game Configuration)
% =============================================

% game_config(人数, 総カード数, 手札枚数, 除外カード)
% 3人: 2♣を除く51枚 → 17枚ずつ
game_config(3, 51, 17, [card(c, 2)]).

% 4人: 52枚全て → 13枚ずつ
game_config(4, 52, 13, []).

% 5人: 2♣と2♦を除く50枚 → 10枚ずつ
game_config(5, 50, 10, [card(c, 2), card(d, 2)]).

% 現在のゲーム人数 (動的に変更可能)
:- dynamic current_players/1.
current_players(4).  % デフォルトは4人

% プレイヤー人数を取得
total_players(N) :-
    current_players(N).

% 手札の枚数を取得
hand_size(Size) :-
    current_players(N),
    game_config(N, _, Size, _).

% 総カード数を取得
total_cards(Total) :-
    current_players(N),
    game_config(N, Total, _, _).

% 除外カードを取得
excluded_cards(Excluded) :-
    current_players(N),
    game_config(N, _, _, Excluded).

% 人数を変更するヘルパー
set_players(N) :-
    game_config(N, _, _, _),  % 有効な人数かチェック
    retractall(current_players(_)),
    assertz(current_players(N)).

% =============================================
% 2. カードの強さと順序 (Card Strength)
% =============================================

% ランクの強さを数値化 (2が最弱、Aが最強)
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

% カードの強さ比較 (同じスートの場合のみ意味を持つ)
% Card1 > Card2 なら true
is_stronger(card(Suit, Rank1), card(Suit, Rank2)) :-
    rank_value(Rank1, Val1),
    rank_value(Rank2, Val2),
    Val1 > Val2.

% =============================================
% 3. カードの得点 (Card Points)
% =============================================

% スペードのクイーン (ブラック・レディ) = 13点
card_point(card(s, q), 13) :- !.

% ハートのカード = 全て1点
card_point(card(h, _), 1) :- !.

% その他のカード = 0点
card_point(_, 0).

% =============================================
% 4. フォローの義務 (Must Follow Suit)
% =============================================

% プレイヤーがカードを出せるかの判定
% can_play(手札, 台札(リード), 出そうとしているカード)

% ケース1: 自分がリードの場合（台札がない）、手札にあるなら何でも出せる
can_play(Hand, nil, Card) :-
    member(Card, Hand).

% ケース2: 台札があり、台札と同じスートを持っている場合 (マストフォロー)
% 出すカードは台札と同じスートでなければならない
can_play(Hand, card(LeadSuit, _), card(PlayerSuit, Rank)) :-
    member(card(PlayerSuit, Rank), Hand),  % 手札にあること
    has_suit(Hand, LeadSuit),              % 手札にリードスートがある
    PlayerSuit == LeadSuit.                % 出すカードがリードスートと一致する

% ケース3: 台札があり、台札と同じスートを持っていない場合
% 手札にあるカードなら何でも出せる (スート不一致でOK)
can_play(Hand, card(LeadSuit, _), card(PlayerSuit, Rank)) :-
    member(card(PlayerSuit, Rank), Hand),  % 手札にあること
    \+ has_suit(Hand, LeadSuit).           % 手札にリードスートがない (否定)

% ヘルパー: 手札の中に指定されたスートがあるか確認
has_suit([], _) :- fail.
has_suit([card(S, _)|_], TargetSuit) :- S == TargetSuit, !.
has_suit([_|Rest], TargetSuit) :- has_suit(Rest, TargetSuit).

% =============================================
% 5. トリックの勝者判定 (Winner of the Trick)
% =============================================

% トリックの勝者を決定する
% カードリストは [card(Suit, Rank), ...] の形式
% LeadSuit: 台札のスート (通常はリストの最初のカードのスート)

determine_winner(PlayedCards, LeadSuit, WinningCard) :-
    % リードスートのカードだけを抽出
    include(is_suit(LeadSuit), PlayedCards, Candidates),
    % その中で最強のカードを見つける
    find_highest(Candidates, WinningCard).

% ヘルパー: 指定スートかどうか
is_suit(TargetSuit, card(TargetSuit, _)).

% ヘルパー: リストの中で最強のカードを見つける
find_highest([Card], Card).
find_highest([Card1, Card2 | Rest], Max) :-
    (is_stronger(Card1, Card2) ->
        find_highest([Card1 | Rest], Max)
    ;
        find_highest([Card2 | Rest], Max)
    ).

% =============================================
% 6. 採点 (Scoring)
% =============================================

% トリックで獲得したカードリストの合計点を計算
calculate_score([], 0).
calculate_score([Card|Rest], TotalPoints) :-
    card_point(Card, P),
    calculate_score(Rest, RestPoints),
    TotalPoints is P + RestPoints.

% =============================================
% 7. ディール (Valid Deal Check)
% =============================================
% 手続き的な「配る処理」ではなく、配られた状態が正当かどうかの検証

validate_deal(PlayerHands) :-
    total_players(NumPlayers),
    total_cards(TotalCards),
    hand_size(HandSize),
    excluded_cards(Excluded),
    length(PlayerHands, NumPlayers),      % 人数分あるか
    flatten(PlayerHands, AllCards),       % 全カードをフラットなリストに
    length(AllCards, TotalCards),         % 合計枚数が正しいか
    is_set(AllCards),                     % 重複がないか
    no_excluded_cards(AllCards, Excluded),% 除外カードが含まれていないか
    check_hand_sizes(PlayerHands, HandSize). % 各自の手札枚数が正しいか

% ヘルパー: 除外カードが含まれていないことを確認
no_excluded_cards(_, []).
no_excluded_cards(AllCards, [Card|Rest]) :-
    \+ member(Card, AllCards),
    no_excluded_cards(AllCards, Rest).

% ヘルパー: 各プレイヤーの手札枚数を確認
check_hand_sizes([], _).
check_hand_sizes([Hand|Rest], ExpectedSize) :-
    length(Hand, ExpectedSize),
    check_hand_sizes(Rest, ExpectedSize).