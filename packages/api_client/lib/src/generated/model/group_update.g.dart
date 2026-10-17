// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupUpdate extends GroupUpdate {
  @override
  final String? name;
  @override
  final String? description;

  factory _$GroupUpdate([void Function(GroupUpdateBuilder)? updates]) =>
      (GroupUpdateBuilder()..update(updates))._build();

  _$GroupUpdate._({this.name, this.description}) : super._();
  @override
  GroupUpdate rebuild(void Function(GroupUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupUpdateBuilder toBuilder() => GroupUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupUpdate &&
        name == other.name &&
        description == other.description;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GroupUpdate')
          ..add('name', name)
          ..add('description', description))
        .toString();
  }
}

class GroupUpdateBuilder implements Builder<GroupUpdate, GroupUpdateBuilder> {
  _$GroupUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  GroupUpdateBuilder() {
    GroupUpdate._defaults(this);
  }

  GroupUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupUpdate other) {
    _$v = other as _$GroupUpdate;
  }

  @override
  void update(void Function(GroupUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupUpdate build() => _build();

  _$GroupUpdate _build() {
    final _$result =
        _$v ?? _$GroupUpdate._(name: name, description: description);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
