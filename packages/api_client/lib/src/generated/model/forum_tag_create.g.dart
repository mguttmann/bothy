// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_tag_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ForumTagCreate extends ForumTagCreate {
  @override
  final String name;
  @override
  final String? color;
  @override
  final bool? restricted;
  @override
  final String? afterTagId;

  factory _$ForumTagCreate([void Function(ForumTagCreateBuilder)? updates]) =>
      (ForumTagCreateBuilder()..update(updates))._build();

  _$ForumTagCreate._({
    required this.name,
    this.color,
    this.restricted,
    this.afterTagId,
  }) : super._();
  @override
  ForumTagCreate rebuild(void Function(ForumTagCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ForumTagCreateBuilder toBuilder() => ForumTagCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ForumTagCreate &&
        name == other.name &&
        color == other.color &&
        restricted == other.restricted &&
        afterTagId == other.afterTagId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jc(_$hash, restricted.hashCode);
    _$hash = $jc(_$hash, afterTagId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ForumTagCreate')
          ..add('name', name)
          ..add('color', color)
          ..add('restricted', restricted)
          ..add('afterTagId', afterTagId))
        .toString();
  }
}

class ForumTagCreateBuilder
    implements Builder<ForumTagCreate, ForumTagCreateBuilder> {
  _$ForumTagCreate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  bool? _restricted;
  bool? get restricted => _$this._restricted;
  set restricted(bool? restricted) => _$this._restricted = restricted;

  String? _afterTagId;
  String? get afterTagId => _$this._afterTagId;
  set afterTagId(String? afterTagId) => _$this._afterTagId = afterTagId;

  ForumTagCreateBuilder() {
    ForumTagCreate._defaults(this);
  }

  ForumTagCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _color = $v.color;
      _restricted = $v.restricted;
      _afterTagId = $v.afterTagId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ForumTagCreate other) {
    _$v = other as _$ForumTagCreate;
  }

  @override
  void update(void Function(ForumTagCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ForumTagCreate build() => _build();

  _$ForumTagCreate _build() {
    final _$result =
        _$v ??
        _$ForumTagCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ForumTagCreate',
            'name',
          ),
          color: color,
          restricted: restricted,
          afterTagId: afterTagId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
