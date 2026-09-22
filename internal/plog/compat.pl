% =============================================
% compat.pl
% ichiban/prolog (ISO準拠・純Go実装) には SWI-Prolog 固有の
% 述語・演算子が無いため、hearts.pl を「無修正のまま」読み込める
% ようにするための互換レイヤ。
% 必ず hearts.pl より先に読み込むこと。
% =============================================

% --- :- dynamic Name/Arity. を通すための前置演算子と no-op 述語 ---
% ichiban/prolog では全ての述語が動的に assertz/retract 可能なため、
% dynamic 宣言は「読み飛ばす」だけでよい。
:- op(1150, fx, dynamic).
:- op(1150, fx, discontiguous).

dynamic(_).
discontiguous(_).

% --- SWI: library(apply) の include/3 ---
include(_, [], []).
include(Goal, [X|Xs], Included) :-
    (   call(Goal, X)
    ->  Included = [X|Rest]
    ;   Included = Rest
    ),
    include(Goal, Xs, Rest).

% --- SWI: library(apply) の exclude/3 ---
exclude(_, [], []).
exclude(Goal, [X|Xs], Excluded) :-
    (   call(Goal, X)
    ->  Excluded = Rest
    ;   Excluded = [X|Rest]
    ),
    exclude(Goal, Xs, Rest).

% --- SWI: library(lists) の flatten/2 ---
flatten(List, Flat) :-
    flatten_(List, [], Flat0),
    Flat = Flat0.

flatten_(Var, Tl, [Var|Tl]) :-
    var(Var),
    !.
flatten_([], Tl, Tl) :- !.
flatten_([Hd|Tl], Tail, List) :-
    !,
    flatten_(Hd, FlatHeadTail, List),
    flatten_(Tl, Tail, FlatHeadTail).
flatten_(NonList, Tl, [NonList|Tl]).

% --- SWI: library(lists) の is_set/1 ---
% sort/2 は ISO 準拠で重複を除去するため、長さが変わらなければ集合。
is_set(List) :-
    sort(List, Sorted),
    length(List, N),
    length(Sorted, N).

% --- SWI: library(lists) の sum_list/2 (AI 側で使用) ---
sum_list([], 0).
sum_list([X|Xs], Sum) :-
    sum_list(Xs, Rest),
    Sum is X + Rest.

% --- SWI: library(lists) の subtract/3 ---
subtract([], _, []).
subtract([X|Xs], Ys, Zs) :-
    (   memberchk(X, Ys)
    ->  Zs = Rest
    ;   Zs = [X|Rest]
    ),
    subtract(Xs, Ys, Rest).

memberchk(X, [Y|Ys]) :-
    (   X = Y
    ->  true
    ;   memberchk(X, Ys)
    ).

% --- SWI: library(lists) の max_list/2, min_list/2 ---
max_list([X|Xs], Max) :-
    max_list_(Xs, X, Max).

max_list_([], Max, Max).
max_list_([X|Xs], Acc, Max) :-
    (   X > Acc
    ->  max_list_(Xs, X, Max)
    ;   max_list_(Xs, Acc, Max)
    ).

min_list([X|Xs], Min) :-
    min_list_(Xs, X, Min).

min_list_([], Min, Min).
min_list_([X|Xs], Acc, Min) :-
    (   X < Acc
    ->  min_list_(Xs, X, Min)
    ;   min_list_(Xs, Acc, Min)
    ).

% --- SWI: library(lists) の numlist/3 ---
numlist(Low, High, []) :-
    Low > High,
    !.
numlist(Low, High, [Low|Rest]) :-
    Next is Low + 1,
    numlist(Next, High, Rest).

% --- SWI: library(lists) の last/2 ---
last([X], X) :- !.
last([_|Xs], X) :- last(Xs, X).

% --- SWI: library(lists) の delete/3 (1つだけでなく全て消す) ---
delete([], _, []).
delete([X|Xs], Y, Zs) :-
    (   X == Y
    ->  Zs = Rest
    ;   Zs = [X|Rest]
    ),
    delete(Xs, Y, Rest).

% --- SWI: library(lists) の intersection/3 ---
intersection([], _, []).
intersection([X|Xs], Ys, Zs) :-
    (   memberchk(X, Ys)
    ->  Zs = [X|Rest]
    ;   Zs = Rest
    ),
    intersection(Xs, Ys, Rest).

% --- SWI: library(lists) の subset/2 ---
subset([], _).
subset([X|Xs], Ys) :-
    memberchk(X, Ys),
    subset(Xs, Ys).

% --- SWI: library(lists) の list_to_set/2 (順序を保った重複除去) ---
list_to_set([], []).
list_to_set([X|Xs], [X|Ys]) :-
    delete(Xs, X, Rest),
    list_to_set(Rest, Ys).

% --- SWI: library(apply) の forall/2 ---
forall(Cond, Action) :-
    \+ ( call(Cond), \+ call(Action) ).

% --- SWI: library(pairs) の pairs_keys_values/3 と pairs_values/2 ---
% keysort/2 と組み合わせて「キーで並べ替える」ために使う。
pairs_values([], []).
pairs_values([_-V|T], [V|Vs]) :-
    pairs_values(T, Vs).

pairs_keys([], []).
pairs_keys([K-_|T], [K|Ks]) :-
    pairs_keys(T, Ks).

% --- リストの先頭 N 要素 ---
take(0, _, []) :- !.
take(_, [], []) :- !.
take(N, [X|Xs], [X|Ys]) :-
    N > 0,
    N1 is N - 1,
    take(N1, Xs, Ys).

% --- 0 始まりの位置 (要素の同一性は ==) ---
index_of(X, List, Index) :-
    index_of_(List, X, 0, Index).

index_of_([X|_], Y, I, I) :-
    X == Y,
    !.
index_of_([_|T], Y, I0, I) :-
    I1 is I0 + 1,
    index_of_(T, Y, I1, I).
