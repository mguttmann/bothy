// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_assist_pick.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchAssistPick extends SearchAssistPick {
  @override
  final String ref;
  @override
  final String reason;

  factory _$SearchAssistPick([
    void Function(SearchAssistPickBuilder)? updates,
  ]) => (SearchAssistPickBuilder()..update(updates))._build();

  _$SearchAssistPick._({required this.ref, required this.reason}) : super._();
  @override
  SearchAssistPick rebuild(void Function(SearchAssistPickBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchAssistPickBuilder toBuilder() =>
      SearchAssistPickBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchAssistPick &&
        ref == other.ref &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, ref.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchAssistPick')
          ..add('ref', ref)
          ..add('reason', reason))
        .toString();
  }
}

class SearchAssistPickBuilder
    implements Builder<SearchAssistPick, SearchAssistPickBuilder> {
  _$SearchAssistPick? _$v;

  String? _ref;
  String? get ref => _$this._ref;
  set ref(String? ref) => _$this._ref = ref;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  SearchAssistPickBuilder() {
    SearchAssistPick._defaults(this);
  }

  SearchAssistPickBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _ref = $v.ref;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchAssistPick other) {
    _$v = other as _$SearchAssistPick;
  }

  @override
  void update(void Function(SearchAssistPickBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchAssistPick build() => _build();

  _$SearchAssistPick _build() {
    final _$result =
        _$v ??
        _$SearchAssistPick._(
          ref: BuiltValueNullFieldError.checkNotNull(
            ref,
            r'SearchAssistPick',
            'ref',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'SearchAssistPick',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
