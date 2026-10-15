// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_tag_update.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ForumTagUpdate extends ForumTagUpdate {
  @override
  final String? name;
  @override
  final String? color;
  @override
  final bool? clearColor;
  @override
  final bool? restricted;

  factory _$ForumTagUpdate([void Function(ForumTagUpdateBuilder)? updates]) =>
      (ForumTagUpdateBuilder()..update(updates))._build();

  _$ForumTagUpdate._({this.name, this.color, this.clearColor, this.restricted})
    : super._();
  @override
  ForumTagUpdate rebuild(void Function(ForumTagUpdateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ForumTagUpdateBuilder toBuilder() => ForumTagUpdateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ForumTagUpdate &&
        name == other.name &&
        color == other.color &&
        clearColor == other.clearColor &&
        restricted == other.restricted;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jc(_$hash, clearColor.hashCode);
    _$hash = $jc(_$hash, restricted.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ForumTagUpdate')
          ..add('name', name)
          ..add('color', color)
          ..add('clearColor', clearColor)
          ..add('restricted', restricted))
        .toString();
  }
}

class ForumTagUpdateBuilder
    implements Builder<ForumTagUpdate, ForumTagUpdateBuilder> {
  _$ForumTagUpdate? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  bool? _clearColor;
  bool? get clearColor => _$this._clearColor;
  set clearColor(bool? clearColor) => _$this._clearColor = clearColor;

  bool? _restricted;
  bool? get restricted => _$this._restricted;
  set restricted(bool? restricted) => _$this._restricted = restricted;

  ForumTagUpdateBuilder() {
    ForumTagUpdate._defaults(this);
  }

  ForumTagUpdateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _color = $v.color;
      _clearColor = $v.clearColor;
      _restricted = $v.restricted;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ForumTagUpdate other) {
    _$v = other as _$ForumTagUpdate;
  }

  @override
  void update(void Function(ForumTagUpdateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ForumTagUpdate build() => _build();

  _$ForumTagUpdate _build() {
    final _$result =
        _$v ??
        _$ForumTagUpdate._(
          name: name,
          color: color,
          clearColor: clearColor,
          restricted: restricted,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
