// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_setting_list.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserSettingList extends UserSettingList {
  @override
  final BuiltList<UserSetting> items;

  factory _$UserSettingList([void Function(UserSettingListBuilder)? updates]) =>
      (UserSettingListBuilder()..update(updates))._build();

  _$UserSettingList._({required this.items}) : super._();
  @override
  UserSettingList rebuild(void Function(UserSettingListBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserSettingListBuilder toBuilder() => UserSettingListBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserSettingList && items == other.items;
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
      r'UserSettingList',
    )..add('items', items)).toString();
  }
}

class UserSettingListBuilder
    implements Builder<UserSettingList, UserSettingListBuilder> {
  _$UserSettingList? _$v;

  ListBuilder<UserSetting>? _items;
  ListBuilder<UserSetting> get items =>
      _$this._items ??= ListBuilder<UserSetting>();
  set items(ListBuilder<UserSetting>? items) => _$this._items = items;

  UserSettingListBuilder() {
    UserSettingList._defaults(this);
  }

  UserSettingListBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserSettingList other) {
    _$v = other as _$UserSettingList;
  }

  @override
  void update(void Function(UserSettingListBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserSettingList build() => _build();

  _$UserSettingList _build() {
    _$UserSettingList _$result;
    try {
      _$result = _$v ?? _$UserSettingList._(items: items.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UserSettingList',
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
