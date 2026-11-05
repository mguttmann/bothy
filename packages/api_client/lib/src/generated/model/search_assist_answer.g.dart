// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_answer.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistAnswer extends SearchAssistAnswer {
  @override
  final BuiltList<SearchAssistPick>? picks;
  @override
  final BuiltList<String>? terms;
  @override
  final BuiltList<CostReport>? costs;

  factory _$SearchAssistAnswer([
    void Function(SearchAssistAnswerBuilder)? updates,
  ]) => (SearchAssistAnswerBuilder()..update(updates))._build();

  _$SearchAssistAnswer._({this.picks, this.terms, this.costs}) : super._();
  @override
  SearchAssistAnswer rebuild(
    void Function(SearchAssistAnswerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SearchAssistAnswerBuilder toBuilder() =>
      SearchAssistAnswerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistAnswer &&
        picks == other.picks &&
        terms == other.terms &&
        costs == other.costs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, picks.hashCode);
    _$hash = $jc(_$hash, terms.hashCode);
    _$hash = $jc(_$hash, costs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistAnswer')
          ..add('picks', picks)
          ..add('terms', terms)
          ..add('costs', costs))
        .toString();
  }
}

class SearchAssistAnswerBuilder
    implements Builder<SearchAssistAnswer, SearchAssistAnswerBuilder> {
  _$SearchAssistAnswer? _$v;

  ListBuilder<SearchAssistPick>? _picks;
  ListBuilder<SearchAssistPick> get picks =>
      _$this._picks ??= ListBuilder<SearchAssistPick>();
  set picks(ListBuilder<SearchAssistPick>? picks) => _$this._picks = picks;

  ListBuilder<String>? _terms;
  ListBuilder<String> get terms => _$this._terms ??= ListBuilder<String>();
  set terms(ListBuilder<String>? terms) => _$this._terms = terms;

  ListBuilder<CostReport>? _costs;
  ListBuilder<CostReport> get costs =>
      _$this._costs ??= ListBuilder<CostReport>();
  set costs(ListBuilder<CostReport>? costs) => _$this._costs = costs;

  SearchAssistAnswerBuilder() {
    SearchAssistAnswer._defaults(this);
  }

  SearchAssistAnswerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _picks = $v.picks?.toBuilder();
      _terms = $v.terms?.toBuilder();
      _costs = $v.costs?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistAnswer other) {
    _$v = other as _$SearchAssistAnswer;
  }

  @override
  void update(void Function(SearchAssistAnswerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistAnswer build() => _build();

  _$SearchAssistAnswer _build() {
    _$SearchAssistAnswer _$result;
    try {
      _$result =
          _$v ??
          _$SearchAssistAnswer._(
            picks: _picks?.build(),
            terms: _terms?.build(),
            costs: _costs?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'picks';
        _picks?.build();
        _$failedField = 'terms';
        _terms?.build();
        _$failedField = 'costs';
        _costs?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchAssistAnswer',
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
