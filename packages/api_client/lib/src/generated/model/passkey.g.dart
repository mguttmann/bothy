// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'passkey.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Passkey extends Passkey {
  @override
  final String id;
  @override
  final String name;
  @override
  final String? aaguid;
  @override
  final bool backupEligible;
  @override
  final DateTime createdAt;
  @override
  final DateTime? lastUsedAt;

  factory _$Passkey([void Function(PasskeyBuilder)? updates]) =>
      (PasskeyBuilder()..update(updates))._build();

  _$Passkey._({
    required this.id,
    required this.name,
    this.aaguid,
    required this.backupEligible,
    required this.createdAt,
    this.lastUsedAt,
  }) : super._();
  @override
  Passkey rebuild(void Function(PasskeyBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PasskeyBuilder toBuilder() => PasskeyBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Passkey &&
        id == other.id &&
        name == other.name &&
        aaguid == other.aaguid &&
        backupEligible == other.backupEligible &&
        createdAt == other.createdAt &&
        lastUsedAt == other.lastUsedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, aaguid.hashCode);
    _$hash = $jc(_$hash, backupEligible.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, lastUsedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Passkey')
          ..add('id', id)
          ..add('name', name)
          ..add('aaguid', aaguid)
          ..add('backupEligible', backupEligible)
          ..add('createdAt', createdAt)
          ..add('lastUsedAt', lastUsedAt))
        .toString();
  }
}

class PasskeyBuilder implements Builder<Passkey, PasskeyBuilder> {
  _$Passkey? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _aaguid;
  String? get aaguid => _$this._aaguid;
  set aaguid(String? aaguid) => _$this._aaguid = aaguid;

  bool? _backupEligible;
  bool? get backupEligible => _$this._backupEligible;
  set backupEligible(bool? backupEligible) =>
      _$this._backupEligible = backupEligible;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _lastUsedAt;
  DateTime? get lastUsedAt => _$this._lastUsedAt;
  set lastUsedAt(DateTime? lastUsedAt) => _$this._lastUsedAt = lastUsedAt;

  PasskeyBuilder() {
    Passkey._defaults(this);
  }

  PasskeyBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _aaguid = $v.aaguid;
      _backupEligible = $v.backupEligible;
      _createdAt = $v.createdAt;
      _lastUsedAt = $v.lastUsedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Passkey other) {
    _$v = other as _$Passkey;
  }

  @override
  void update(void Function(PasskeyBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Passkey build() => _build();

  _$Passkey _build() {
    final _$result =
        _$v ??
        _$Passkey._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'Passkey', 'id'),
          name: BuiltValueNullFieldError.checkNotNull(name, r'Passkey', 'name'),
          aaguid: aaguid,
          backupEligible: BuiltValueNullFieldError.checkNotNull(
            backupEligible,
            r'Passkey',
            'backupEligible',
          ),
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'Passkey',
            'createdAt',
          ),
          lastUsedAt: lastUsedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
