// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'space.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Space extends Space {
  @override
  final String id;
  @override
  final String name;
  @override
  final String? icon;
  @override
  final String? color;
  @override
  final String sortKey;
  @override
  final String? defaultOperatorAgentId;
  @override
  final DateTime? archivedAt;
  @override
  final int version;
  @override
  final DateTime createdAt;

  factory _$Space([void Function(SpaceBuilder)? updates]) =>
      (SpaceBuilder()..update(updates))._build();

  _$Space._({
    required this.id,
    required this.name,
    this.icon,
    this.color,
    required this.sortKey,
    this.defaultOperatorAgentId,
    this.archivedAt,
    required this.version,
    required this.createdAt,
  }) : super._();
  @override
  Space rebuild(void Function(SpaceBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SpaceBuilder toBuilder() => SpaceBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Space &&
        id == other.id &&
        name == other.name &&
        icon == other.icon &&
        color == other.color &&
        sortKey == other.sortKey &&
        defaultOperatorAgentId == other.defaultOperatorAgentId &&
        archivedAt == other.archivedAt &&
        version == other.version &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, icon.hashCode);
    _$hash = $jc(_$hash, color.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, defaultOperatorAgentId.hashCode);
    _$hash = $jc(_$hash, archivedAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Space')
          ..add('id', id)
          ..add('name', name)
          ..add('icon', icon)
          ..add('color', color)
          ..add('sortKey', sortKey)
          ..add('defaultOperatorAgentId', defaultOperatorAgentId)
          ..add('archivedAt', archivedAt)
          ..add('version', version)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class SpaceBuilder implements Builder<Space, SpaceBuilder> {
  _$Space? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _icon;
  String? get icon => _$this._icon;
  set icon(String? icon) => _$this._icon = icon;

  String? _color;
  String? get color => _$this._color;
  set color(String? color) => _$this._color = color;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  String? _defaultOperatorAgentId;
  String? get defaultOperatorAgentId => _$this._defaultOperatorAgentId;
  set defaultOperatorAgentId(String? defaultOperatorAgentId) =>
      _$this._defaultOperatorAgentId = defaultOperatorAgentId;

  DateTime? _archivedAt;
  DateTime? get archivedAt => _$this._archivedAt;
  set archivedAt(DateTime? archivedAt) => _$this._archivedAt = archivedAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  SpaceBuilder() {
    Space._defaults(this);
  }

  SpaceBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _icon = $v.icon;
      _color = $v.color;
      _sortKey = $v.sortKey;
      _defaultOperatorAgentId = $v.defaultOperatorAgentId;
      _archivedAt = $v.archivedAt;
      _version = $v.version;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Space other) {
    _$v = other as _$Space;
  }

  @override
  void update(void Function(SpaceBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Space build() => _build();

  _$Space _build() {
    final _$result =
        _$v ??
        _$Space._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Space', 'id'),
          name: BuiltValueNullFieldError.checkNotNull(name, r'Space', 'name'),
          icon: icon,
          color: color,
          sortKey: BuiltValueNullFieldError.checkNotNull(
            sortKey,
            r'Space',
            'sortKey',
          ),
          defaultOperatorAgentId: defaultOperatorAgentId,
          archivedAt: archivedAt,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'Space',
            'version',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'Space',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
