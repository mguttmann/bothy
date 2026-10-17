// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_tag.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ForumTag extends ForumTag {
  @override
  final String id;
  @override
  final String channelId;
  @override
  final String name;
  @override
  final String? color;
  @override
  final String sortKey;
  @override
  final bool restricted;

  factory _$ForumTag([void Function(ForumTagBuilder)? updates]) =>
      (ForumTagBuilder()..update(updates))._build();

  _$ForumTag._({
    required this.id,
    required this.channelId,
    required this.name,
    this.color,
    required this.sortKey,
    required this.restricted,
  }) : super._();
  @override
  ForumTag rebuild(void Function(ForumTagBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ForumTagBuilder toBuilder() => ForumTagBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ForumTag &&
        id == other.id &&
        channelId == other.channelId &&
        name == other.name &&
        color == other.color &&
        sortKey == other.sortKey &&
        restricted == other.restricted;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, restricted.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ForumTag')
          ..add('id', id)
          ..add('channelId', channelId)
          ..add('name', name)
          ..add('color', color)
          ..add('sortKey', sortKey)
          ..add('restricted', restricted))
        .toString();
  }
}

class ForumTagBuilder implements Builder<ForumTag, ForumTagBuilder> {
  _$ForumTag? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  bool? _restricted;
  bool? get restricted => _$this._restricted;
  set restricted(bool? restricted) => _$this._restricted = restricted;

  ForumTagBuilder() {
    ForumTag._defaults(this);
  }

  ForumTagBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _channelId = $v.channelId;
      _name = $v.name;
      _color = $v.color;
      _sortKey = $v.sortKey;
      _restricted = $v.restricted;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ForumTag other) {
    _$v = other as _$ForumTag;
  }

  @override
  void update(void Function(ForumTagBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ForumTag build() => _build();

  _$ForumTag _build() {
    final _$result =
        _$v ??
        _$ForumTag._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'ForumTag', 'id'),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ForumTag',
            'channelId',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ForumTag',
            'name',
          ),
          color: color,
          sortKey: BuiltValueNullFieldError.checkNotNull(
            sortKey,
            r'ForumTag',
            'sortKey',
          ),
          restricted: BuiltValueNullFieldError.checkNotNull(
            restricted,
            r'ForumTag',
            'restricted',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
