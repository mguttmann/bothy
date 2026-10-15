// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GdprRequestPage extends GdprRequestPage {
  @override
  final BuiltList<GdprRequest> items;
  @override
  final String? nextCursor;

  factory _$GdprRequestPage([void Function(GdprRequestPageBuilder)? updates]) =>
      (GdprRequestPageBuilder()..update(updates))._build();

  _$GdprRequestPage._({required this.items, this.nextCursor}) : super._();
  @override
  GdprRequestPage rebuild(void Function(GdprRequestPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GdprRequestPageBuilder toBuilder() => GdprRequestPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprRequestPage &&
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
    return (newBuiltValueToStringHelper(r'GdprRequestPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class GdprRequestPageBuilder
    implements Builder<GdprRequestPage, GdprRequestPageBuilder> {
  _$GdprRequestPage? _$v;

  ListBuilder<GdprRequest>? _items;
  ListBuilder<GdprRequest> get items =>
      _$this._items ??= ListBuilder<GdprRequest>();
  set items(ListBuilder<GdprRequest>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  GdprRequestPageBuilder() {
    GdprRequestPage._defaults(this);
  }

  GdprRequestPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprRequestPage other) {
    _$v = other as _$GdprRequestPage;
  }

  @override
  void update(void Function(GdprRequestPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprRequestPage build() => _build();

  _$GdprRequestPage _build() {
    _$GdprRequestPage _$result;
    try {
      _$result =
          _$v ??
          _$GdprRequestPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GdprRequestPage',
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
