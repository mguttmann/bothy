// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_hit_segments_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchHitSegmentsInner extends SearchHitSegmentsInner {
  @override
  final String text;
  @override
  final bool match;

  factory _$SearchHitSegmentsInner([
    void Function(SearchHitSegmentsInnerBuilder)? updates,
  ]) => (SearchHitSegmentsInnerBuilder()..update(updates))._build();

  _$SearchHitSegmentsInner._({required this.text, required this.match})
    : super._();
  @override
  SearchHitSegmentsInner rebuild(
    void Function(SearchHitSegmentsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  SearchHitSegmentsInnerBuilder toBuilder() =>
      SearchHitSegmentsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchHitSegmentsInner &&
        text == other.text &&
        match == other.match;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jc(_$hash, match.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchHitSegmentsInner')
          ..add('text', text)
          ..add('match', match))
        .toString();
  }
}

class SearchHitSegmentsInnerBuilder
    implements Builder<SearchHitSegmentsInner, SearchHitSegmentsInnerBuilder> {
  _$SearchHitSegmentsInner? _$v;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  bool? _match;
  bool? get match => _$this._match;
  set match(bool? match) => _$this._match = match;

  SearchHitSegmentsInnerBuilder() {
    SearchHitSegmentsInner._defaults(this);
  }

  SearchHitSegmentsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _text = $v.text;
      _match = $v.match;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchHitSegmentsInner other) {
    _$v = other as _$SearchHitSegmentsInner;
  }

  @override
  void update(void Function(SearchHitSegmentsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchHitSegmentsInner build() => _build();

  _$SearchHitSegmentsInner _build() {
    final _$result =
        _$v ??
        _$SearchHitSegmentsInner._(
          text: BuiltValueNullFieldError.checkNotNull(
            text,
            r'SearchHitSegmentsInner',
            'text',
          ),
          match: BuiltValueNullFieldError.checkNotNull(
            match,
            r'SearchHitSegmentsInner',
            'match',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
