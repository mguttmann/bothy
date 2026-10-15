// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_hit_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchHitPage extends SearchHitPage {
  @override
  final BuiltList<SearchHit> items;
  @override
  final String? nextCursor;
  @override
  final bool? candidatesCapped;

  factory _$SearchHitPage([void Function(SearchHitPageBuilder)? updates]) =>
      (SearchHitPageBuilder()..update(updates))._build();

  _$SearchHitPage._({
    required this.items,
    this.nextCursor,
    this.candidatesCapped,
  }) : super._();
  @override
  SearchHitPage rebuild(void Function(SearchHitPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchHitPageBuilder toBuilder() => SearchHitPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchHitPage &&
        items == other.items &&
        nextCursor == other.nextCursor &&
        candidatesCapped == other.candidatesCapped;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jc(_$hash, candidatesCapped.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchHitPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor)
          ..add('candidatesCapped', candidatesCapped))
        .toString();
  }
}

class SearchHitPageBuilder
    implements Builder<SearchHitPage, SearchHitPageBuilder> {
  _$SearchHitPage? _$v;

  ListBuilder<SearchHit>? _items;
  ListBuilder<SearchHit> get items =>
      _$this._items ??= ListBuilder<SearchHit>();
  set items(ListBuilder<SearchHit>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  bool? _candidatesCapped;
  bool? get candidatesCapped => _$this._candidatesCapped;
  set candidatesCapped(bool? candidatesCapped) =>
      _$this._candidatesCapped = candidatesCapped;

  SearchHitPageBuilder() {
    SearchHitPage._defaults(this);
  }

  SearchHitPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _candidatesCapped = $v.candidatesCapped;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchHitPage other) {
    _$v = other as _$SearchHitPage;
  }

  @override
  void update(void Function(SearchHitPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchHitPage build() => _build();

  _$SearchHitPage _build() {
    _$SearchHitPage _$result;
    try {
      _$result =
          _$v ??
          _$SearchHitPage._(
            items: items.build(),
            nextCursor: nextCursor,
            candidatesCapped: candidatesCapped,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchHitPage',
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
