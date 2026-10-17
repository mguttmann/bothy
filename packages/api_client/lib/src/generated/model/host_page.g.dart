// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostPage extends HostPage {
  @override
  final BuiltList<Host> items;
  @override
  final String? nextCursor;

  factory _$HostPage([void Function(HostPageBuilder)? updates]) =>
      (HostPageBuilder()..update(updates))._build();

  _$HostPage._({required this.items, this.nextCursor}) : super._();
  @override
  HostPage rebuild(void Function(HostPageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostPageBuilder toBuilder() => HostPageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostPage &&
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
    return (newBuiltValueToStringHelper(r'HostPage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class HostPageBuilder implements Builder<HostPage, HostPageBuilder> {
  _$HostPage? _$v;

  ListBuilder<Host>? _items;
  ListBuilder<Host> get items => _$this._items ??= ListBuilder<Host>();
  set items(ListBuilder<Host>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  HostPageBuilder() {
    HostPage._defaults(this);
  }

  HostPageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostPage other) {
    _$v = other as _$HostPage;
  }

  @override
  void update(void Function(HostPageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostPage build() => _build();

  _$HostPage _build() {
    _$HostPage _$result;
    try {
      _$result =
          _$v ?? _$HostPage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostPage',
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
