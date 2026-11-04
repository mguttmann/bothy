// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_query.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistQuery extends SearchAssistQuery {
  @override
  final String q;
  @override
  final BuiltList<SearchAssistTurn>? history;

  factory _$SearchAssistQuery([
    void Function(SearchAssistQueryBuilder)? updates,
  ]) => (SearchAssistQueryBuilder()..update(updates))._build();

  _$SearchAssistQuery._({required this.q, this.history}) : super._();
  @override
  SearchAssistQuery rebuild(void Function(SearchAssistQueryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchAssistQueryBuilder toBuilder() =>
      SearchAssistQueryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistQuery &&
        q == other.q &&
        history == other.history;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, q.hashCode);
    _$hash = $jc(_$hash, history.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistQuery')
          ..add('q', q)
          ..add('history', history))
        .toString();
  }
}

class SearchAssistQueryBuilder
    implements Builder<SearchAssistQuery, SearchAssistQueryBuilder> {
  _$SearchAssistQuery? _$v;

  String? _q;
  String? get q => _$this._q;
  set q(String? q) => _$this._q = q;

  ListBuilder<SearchAssistTurn>? _history;
  ListBuilder<SearchAssistTurn> get history =>
      _$this._history ??= ListBuilder<SearchAssistTurn>();
  set history(ListBuilder<SearchAssistTurn>? history) =>
      _$this._history = history;

  SearchAssistQueryBuilder() {
    SearchAssistQuery._defaults(this);
  }

  SearchAssistQueryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _q = $v.q;
      _history = $v.history?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistQuery other) {
    _$v = other as _$SearchAssistQuery;
  }

  @override
  void update(void Function(SearchAssistQueryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistQuery build() => _build();

  _$SearchAssistQuery _build() {
    _$SearchAssistQuery _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistQuery._(
            q: BuiltValueNullFieldError.checkNotNull(
              q,
              r'SearchAssistQuery',
              'q',
            ),
            history: _history?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'history';
        _history?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistQuery',
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
