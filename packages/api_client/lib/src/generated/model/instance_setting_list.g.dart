// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_setting_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceSettingList extends InstanceSettingList {
  @override
  final BuiltList<InstanceSetting> items;

  factory _$InstanceSettingList([
    void Function(InstanceSettingListBuilder)? updates,
  ]) => (InstanceSettingListBuilder()..update(updates))._build();

  _$InstanceSettingList._({required this.items}) : super._();
  @override
  InstanceSettingList rebuild(
    void Function(InstanceSettingListBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InstanceSettingListBuilder toBuilder() =>
      InstanceSettingListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceSettingList && items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'InstanceSettingList',
    )..add('items', items)).toString();
  }
}

class InstanceSettingListBuilder
    implements Builder<InstanceSettingList, InstanceSettingListBuilder> {
  _$InstanceSettingList? _$v;

  ListBuilder<InstanceSetting>? _items;
  ListBuilder<InstanceSetting> get items =>
      _$this._items ??= ListBuilder<InstanceSetting>();
  set items(ListBuilder<InstanceSetting>? items) => _$this._items = items;

  InstanceSettingListBuilder() {
    InstanceSettingList._defaults(this);
  }

  InstanceSettingListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceSettingList other) {
    _$v = other as _$InstanceSettingList;
  }

  @override
  void update(void Function(InstanceSettingListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceSettingList build() => _build();

  _$InstanceSettingList _build() {
    _$InstanceSettingList _$result;
    try {
      _$result = _$v ?? _$InstanceSettingList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InstanceSettingList',
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
