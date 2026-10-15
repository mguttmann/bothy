// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'device_page.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DevicePage extends DevicePage {
  @override
  final BuiltList<Device> items;
  @override
  final String? nextCursor;

  factory _$DevicePage([void Function(DevicePageBuilder)? updates]) =>
      (DevicePageBuilder()..update(updates))._build();

  _$DevicePage._({required this.items, this.nextCursor}) : super._();
  @override
  DevicePage rebuild(void Function(DevicePageBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DevicePageBuilder toBuilder() => DevicePageBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DevicePage &&
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
    return (newBuiltValueToStringHelper(r'DevicePage')
          ..add('items', items)
          ..add('nextCursor', nextCursor))
        .toString();
  }
}

class DevicePageBuilder implements Builder<DevicePage, DevicePageBuilder> {
  _$DevicePage? _$v;

  ListBuilder<Device>? _items;
  ListBuilder<Device> get items => _$this._items ??= ListBuilder<Device>();
  set items(ListBuilder<Device>? items) => _$this._items = items;

  String? _nextCursor;
  String? get nextCursor => _$this._nextCursor;
  set nextCursor(String? nextCursor) => _$this._nextCursor = nextCursor;

  DevicePageBuilder() {
    DevicePage._defaults(this);
  }

  DevicePageBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _nextCursor = $v.nextCursor;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DevicePage other) {
    _$v = other as _$DevicePage;
  }

  @override
  void update(void Function(DevicePageBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DevicePage build() => _build();

  _$DevicePage _build() {
    _$DevicePage _$result;
    try {
      _$result =
          _$v ?? _$DevicePage._(items: items.build(), nextCursor: nextCursor);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'DevicePage',
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
