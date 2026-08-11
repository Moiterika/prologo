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
