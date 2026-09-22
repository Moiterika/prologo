% =============================================
% daifugo_ai.pl
%
% CPU の思考。「合法手のなかから一様にランダムに選ぶ」だけ。
%
% Prolog には乱数が無いので、Go から渡した Seed (非負整数) を
% 合法手の本数で割った余りを使って選ぶ。どの手が合法かの判断は
% daifugo_rules.pl の resolve_play/6 に任せてあり、ここには
% ルールの知識を置かない。
%
% 読み込み順: compat.pl -> daifugo.pl -> daifugo_rules.pl -> daifugo_ai.pl
% =============================================

% =============================================
% 1. 出せる手の列挙
% =============================================
% 手札から作れるカードの組を作り、resolve_play/6 で通るものだけを残す。
%
% 候補の作り方は 2 通りだけ。
%   - 同じ数字のカードの組み合わせ (+ ジョーカー)
%   - 同じマークで連続する窓に入るカード全部 (+ 足りない分のジョーカー)
% 手札の全部分集合を試すと組み合わせ爆発するので、この形に絞っている。
% 「持っている札をあえてジョーカーで置き換える」ような手は作らない。

legal_plays(Hand, Field, Rev, Plays) :-
    findall(Cards,
            ( candidate(Hand, Cards),
              can_play(Hand, Cards, Field, Rev) ),
            Plays0),
    normalize_plays(Plays0, Plays).

% 同じ組み合わせが別経路で複数回出てくるので、並べ替えてから重複を除く。
normalize_plays(Plays0, Plays) :-
    findall(Sorted, ( member(P, Plays0), sort(P, Sorted) ), Sorted0),
    list_to_set(Sorted0, Plays).

% --- 候補: 同じ数字 ---
candidate(Hand, Cards) :-
    split_jokers(Hand, Jokers, Reals),
    findall(R, member(card(_, R), Reals), Ranks0),
    list_to_set(Ranks0, Ranks),
    member(Rank, Ranks),
    findall(C, ( member(C, Reals), C = card(_, Rank) ), Same),
    sub_set(Same, Picked),
    Picked \== [],
    sub_set(Jokers, UsedJokers),
    append(Picked, UsedJokers, Cards).

% --- 候補: ジョーカーだけで出す ---
candidate(Hand, Cards) :-
    split_jokers(Hand, Jokers, _),
    sub_set(Jokers, Cards),
    Cards \== [].

% --- 候補: 階段 ---
candidate(Hand, Cards) :-
    split_jokers(Hand, Jokers, Reals),
    length(Jokers, JokerN),
    suit(Suit),
    findall(Pos-C,
            ( member(C, Reals),
              C = card(Suit, R),
              base_strength(R, Pos) ),
            Pairs),
    Pairs \== [],
    between(3, 13, Len),
    LowMax is 14 - Len,
    between(1, LowMax, Low),
    High is Low + Len - 1,
    findall(C,
            ( member(Pos-C, Pairs),
              Pos >= Low,
              Pos =< High ),
            InWindow),
    length(InWindow, Have),
    Have >= 1,
    Need is Len - Have,
    Need >= 0,
    Need =< JokerN,
    take(Need, Jokers, UsedJokers),
    append(InWindow, UsedJokers, Cards).

% 部分集合 (元の並び順は保つ)
sub_set([], []).
sub_set([X|Xs], [X|Ys]) :- sub_set(Xs, Ys).
sub_set([_|Xs], Ys) :- sub_set(Xs, Ys).

% =============================================
% 2. ランダムな選択
% =============================================

% choose_play(手札, 場, 革命中か, パスできるか, Seed, 選んだ手)
%   選んだ手は pass か play(カードのリスト)。
%
% 場が空のときはパスできない (親は必ず何か出す)。
% 出せる手が無ければ pass になる。
choose_play(Hand, Field, Rev, CanPass, Seed, Choice) :-
    legal_plays(Hand, Field, Rev, Plays),
    choices(Plays, Field, CanPass, Choices),
    Choices = [_|_],
    pick(Choices, Seed, Choice).

% パスを選択肢に加えるか
choices(Plays, Field, CanPass, Choices) :-
    findall(play(P), member(P, Plays), PlayChoices),
    (   Field == nil
    ->  Choices = PlayChoices          % 親番はパスできない
    ;   CanPass == false
    ->  Choices = PlayChoices
    ;   append(PlayChoices, [pass], Choices)
    ).

% Seed を選択肢の本数で割った余りで選ぶ
pick(Choices, Seed, Choice) :-
    length(Choices, N),
    N > 0,
    Index is Seed mod N,
    nth0(Index, Choices, Choice).

% =============================================
% 3. カード交換で渡す札を選ぶ
% =============================================
% 下位のプレイヤーは「強いカード」を必ず渡す。強さの順は革命前の基準。
% 上位のプレイヤーは好きに選んでよいので、ここでは弱いカードを渡す。

% strongest_cards(手札, 枚数, 渡すカード)
strongest_cards(Hand, Count, Cards) :-
    order_by_strength(Hand, Ordered),
    reverse_(Ordered, Descending),
    take(Count, Descending, Cards).

% weakest_cards(手札, 枚数, 渡すカード)
weakest_cards(Hand, Count, Cards) :-
    order_by_strength(Hand, Ordered),
    take(Count, Ordered, Cards).

order_by_strength(Hand, Ordered) :-
    findall(V-C,
            ( member(C, Hand),
              card_strength(C, false, V) ),
            Pairs),
    keysort(Pairs, Sorted),
    pairs_values(Sorted, Ordered).

reverse_(List, Reversed) :-
    reverse_(List, [], Reversed).

reverse_([], Acc, Acc).
reverse_([X|Xs], Acc, R) :-
    reverse_(Xs, [X|Acc], R).
