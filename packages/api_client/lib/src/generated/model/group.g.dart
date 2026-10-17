// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Group extends Group {
  @override
  final String id;
  @override
  final String name;
  @override
  final String description;
  @override
  final GroupSource source_;
  @override
  final String? providerId;
  @override
  final String? externalRef;
  @override
  final int memberCount;
  @override
  final DateTime createdAt;

  factory _$Group([void Function(GroupBuilder)? updates]) =>
      (GroupBuilder()..update(updates))._build();

  _$Group._({
    required this.id,
    required this.name,
    required this.description,
    required this.source_,
    this.providerId,
    this.externalRef,
    required this.memberCount,
    required this.createdAt,
  }) : super._();
  @override
  Group rebuild(void Function(GroupBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupBuilder toBuilder() => GroupBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Group &&
        id == other.id &&
        name == other.name &&
        description == other.description &&
        source_ == other.source_ &&
        providerId == other.providerId &&
        externalRef == other.externalRef &&
        memberCount == other.memberCount &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, externalRef.hashCode);
    _$hash = $jc(_$hash, memberCount.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Group')
          ..add('id', id)
          ..add('name', name)
          ..add('description', description)
          ..add('source_', source_)
          ..add('providerId', providerId)
          ..add('externalRef', externalRef)
          ..add('memberCount', memberCount)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class GroupBuilder implements Builder<Group, GroupBuilder> {
  _$Group? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  GroupSource? _source_;
  GroupSource? get source_ => _$this._source_;
  set source_(GroupSource? source_) => _$this._source_ = source_;

  String? _providerId;
  String? get providerId => _$this._providerId;
  set providerId(String? providerId) => _$this._providerId = providerId;

  String? _externalRef;
  String? get externalRef => _$this._externalRef;
  set externalRef(String? externalRef) => _$this._externalRef = externalRef;

  int? _memberCount;
  int? get memberCount => _$this._memberCount;
  set memberCount(int? memberCount) => _$this._memberCount = memberCount;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  GroupBuilder() {
    Group._defaults(this);
  }

  GroupBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _description = $v.description;
      _source_ = $v.source_;
      _providerId = $v.providerId;
      _externalRef = $v.externalRef;
      _memberCount = $v.memberCount;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Group other) {
    _$v = other as _$Group;
  }

  @override
  void update(void Function(GroupBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Group build() => _build();

  _$Group _build() {
    final _$result =
        _$v ??
        _$Group._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Group', 'id'),
          name: BuiltValueNullFieldError.checkNotNull(name, r'Group', 'name'),
          description: BuiltValueNullFieldError.checkNotNull(
            description,
            r'Group',
            'description',
          ),
          source_: BuiltValueNullFieldError.checkNotNull(
            source_,
            r'Group',
            'source_',
          ),
          providerId: providerId,
          externalRef: externalRef,
          memberCount: BuiltValueNullFieldError.checkNotNull(
            memberCount,
            r'Group',
            'memberCount',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'Group',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
