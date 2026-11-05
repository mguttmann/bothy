// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_turn.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistTurn extends SearchAssistTurn {
  @override
  final String q;
  @override
  final BuiltList<String>? terms;
  @override
  final BuiltList<String>? threadIds;

  factory _$SearchAssistTurn([
    void Function(SearchAssistTurnBuilder)? updates,
  ]) => (SearchAssistTurnBuilder()..update(updates))._build();

  _$SearchAssistTurn._({required this.q, this.terms, this.threadIds})
    : super._();
  @override
  SearchAssistTurn rebuild(void Function(SearchAssistTurnBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchAssistTurnBuilder toBuilder() =>
      SearchAssistTurnBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistTurn &&
        q == other.q &&
        terms == other.terms &&
        threadIds == other.threadIds;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, q.hashCode);
    _$hash = $jc(_$hash, terms.hashCode);
    _$hash = $jc(_$hash, threadIds.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistTurn')
          ..add('q', q)
          ..add('terms', terms)
          ..add('threadIds', threadIds))
        .toString();
  }
}

class SearchAssistTurnBuilder
    implements Builder<SearchAssistTurn, SearchAssistTurnBuilder> {
  _$SearchAssistTurn? _$v;

  String? _q;
  String? get q => _$this._q;
  set q(String? q) => _$this._q = q;

  ListBuilder<String>? _terms;
  ListBuilder<String> get terms => _$this._terms ??= ListBuilder<String>();
  set terms(ListBuilder<String>? terms) => _$this._terms = terms;

  ListBuilder<String>? _threadIds;
  ListBuilder<String> get threadIds =>
      _$this._threadIds ??= ListBuilder<String>();
  set threadIds(ListBuilder<String>? threadIds) =>
      _$this._threadIds = threadIds;

  SearchAssistTurnBuilder() {
    SearchAssistTurn._defaults(this);
  }

  SearchAssistTurnBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _q = $v.q;
      _terms = $v.terms?.toBuilder();
      _threadIds = $v.threadIds?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistTurn other) {
    _$v = other as _$SearchAssistTurn;
  }

  @override
  void update(void Function(SearchAssistTurnBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistTurn build() => _build();

  _$SearchAssistTurn _build() {
    _$SearchAssistTurn _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistTurn._(
            q: BuiltValueNullFieldError.checkNotNull(
              q,
              r'SearchAssistTurn',
              'q',
            ),
            terms: _terms?.build(),
            threadIds: _threadIds?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'terms';
        _terms?.build();
        _$failedField = 'threadIds';
        _threadIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistTurn',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
