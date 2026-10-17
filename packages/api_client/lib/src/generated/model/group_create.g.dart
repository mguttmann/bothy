// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'group_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GroupCreate extends GroupCreate {
  @override
  final String name;
  @override
  final String? description;
  @override
  final GroupSource? source_;
  @override
  final String? providerId;
  @override
  final String? externalRef;

  factory _$GroupCreate([void Function(GroupCreateBuilder)? updates]) =>
      (GroupCreateBuilder()..update(updates))._build();

  _$GroupCreate._({
    required this.name,
    this.description,
    this.source_,
    this.providerId,
    this.externalRef,
  }) : super._();
  @override
  GroupCreate rebuild(void Function(GroupCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GroupCreateBuilder toBuilder() => GroupCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GroupCreate &&
        name == other.name &&
        description == other.description &&
        source_ == other.source_ &&
        providerId == other.providerId &&
        externalRef == other.externalRef;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jc(_$hash, providerId.hashCode);
    _$hash = $jc(_$hash, externalRef.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GroupCreate')
          ..add('name', name)
          ..add('description', description)
          ..add('source_', source_)
          ..add('providerId', providerId)
          ..add('externalRef', externalRef))
        .toString();
  }
}

class GroupCreateBuilder implements Builder<GroupCreate, GroupCreateBuilder> {
  _$GroupCreate? _$v;

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

  GroupCreateBuilder() {
    GroupCreate._defaults(this);
  }

  GroupCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _description = $v.description;
      _source_ = $v.source_;
      _providerId = $v.providerId;
      _externalRef = $v.externalRef;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GroupCreate other) {
    _$v = other as _$GroupCreate;
  }

  @override
  void update(void Function(GroupCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GroupCreate build() => _build();

  _$GroupCreate _build() {
    final _$result =
        _$v ??
        _$GroupCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'GroupCreate',
            'name',
          ),
          description: description,
          source_: source_,
          providerId: providerId,
          externalRef: externalRef,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
