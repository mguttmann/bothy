// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SpaceUpdate extends SpaceUpdate {
  @override
  final String? name;
  @override
  final String? icon;
  @override
  final String? color;

  factory _$SpaceUpdate([void Function(SpaceUpdateBuilder)? updates]) =>
      (SpaceUpdateBuilder()..update(updates))._build();

  _$SpaceUpdate._({this.name, this.icon, this.color}) : super._();
  @override
  SpaceUpdate rebuild(void Function(SpaceUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpaceUpdateBuilder toBuilder() => SpaceUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SpaceUpdate &&
        name == other.name &&
        icon == other.icon &&
        color == other.color;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, icon.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SpaceUpdate')
          ..add('name', name)
          ..add('icon', icon)
          ..add('color', color))
        .toString();
  }
}

class SpaceUpdateBuilder implements Builder<SpaceUpdate, SpaceUpdateBuilder> {
  _$SpaceUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _icon;
  String? get icon => _$this._icon;
  set icon(String? icon) => _$this._icon = icon;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  SpaceUpdateBuilder() {
    SpaceUpdate._defaults(this);
  }

  SpaceUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _icon = $v.icon;
      _color = $v.color;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SpaceUpdate other) {
    _$v = other as _$SpaceUpdate;
  }

  @override
  void update(void Function(SpaceUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SpaceUpdate build() => _build();

  _$SpaceUpdate _build() {
    final _$result =
        _$v ?? _$SpaceUpdate._(name: name, icon: icon, color: color);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
