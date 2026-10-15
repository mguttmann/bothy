// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_token_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ApiTokenPage extends ApiTokenPage {
  @override
  final BuiltList<ApiToken> items;
  @override
  final String? nextCursor;

  factory _$ApiTokenPage([void Function(ApiTokenPageBuilder)? updates]) =>
      (ApiTokenPageBuilder()..update(updates))._build();

  _$ApiTokenPage._({required this.items, this.nextCursor}) : super._();
  @override
  ApiTokenPage rebuild(void Function(ApiTokenPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ApiTokenPageBuilder toBuilder() => ApiTokenPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ApiTokenPage &&
        items == other.items &&
        nextCursor == other.nextCursor;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jc(_$hash, nextCursor.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ApiTokenPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class ApiTokenPageBuilder
    implements Builder<ApiTokenPage, ApiTokenPageBuilder> {
  _$ApiTokenPage? _$v;

  ListBuilder<ApiToken>? _items;
  ListBuilder<ApiToken> get items => _$this._items ??= ListBuilder<ApiToken>();
  set items(ListBuilder<ApiToken>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  ApiTokenPageBuilder() {
    ApiTokenPage._defaults(this);
  }

  ApiTokenPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ApiTokenPage other) {
    _$v = other as _$ApiTokenPage;
  }

  @override
  void update(void Function(ApiTokenPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ApiTokenPage build() => _build();

  _$ApiTokenPage _build() {
    _$ApiTokenPage _$result;
    try {
      _$result =
          _$v ?? _$ApiTokenPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ApiTokenPage',
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
