// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SpaceCreate extends SpaceCreate {
  @override
  final String name;
  @override
  final String? icon;
  @override
  final String? color;
  @override
  final String? afterSpaceId;

  factory _$SpaceCreate([void Function(SpaceCreateBuilder)? updates]) =>
      (SpaceCreateBuilder()..update(updates))._build();

  _$SpaceCreate._({
    required this.name,
    this.icon,
    this.color,
    this.afterSpaceId,
  }) : super._();
  @override
  SpaceCreate rebuild(void Function(SpaceCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpaceCreateBuilder toBuilder() => SpaceCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SpaceCreate &&
        name == other.name &&
        icon == other.icon &&
        color == other.color &&
        afterSpaceId == other.afterSpaceId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, icon.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jc(_$hash, afterSpaceId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SpaceCreate')
          ..add('name', name)
          ..add('icon', icon)
          ..add('color', color)
          ..add('afterSpaceId', afterSpaceId))
        .toString();
  }
}

class SpaceCreateBuilder implements Builder<SpaceCreate, SpaceCreateBuilder> {
  _$SpaceCreate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _icon;
  String? get icon => _$this._icon;
  set icon(String? icon) => _$this._icon = icon;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  String? _afterSpaceId;
  String? get afterSpaceId => _$this._afterSpaceId;
  set afterSpaceId(String? afterSpaceId) => _$this._afterSpaceId = afterSpaceId;

  SpaceCreateBuilder() {
    SpaceCreate._defaults(this);
  }

  SpaceCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _icon = $v.icon;
      _color = $v.color;
      _afterSpaceId = $v.afterSpaceId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SpaceCreate other) {
    _$v = other as _$SpaceCreate;
  }

  @override
  void update(void Function(SpaceCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SpaceCreate build() => _build();

  _$SpaceCreate _build() {
    final _$result =
        _$v ??
        _$SpaceCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'SpaceCreate',
            'name',
          ),
          icon: icon,
          color: color,
          afterSpaceId: afterSpaceId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
