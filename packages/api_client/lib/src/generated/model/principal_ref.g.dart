// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'principal_ref.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PrincipalRef extends PrincipalRef {
  @override
  final String id;
  @override
  final PrincipalKind kind;
  @override
  final String handle;
  @override
  final String displayName;
  @override
  final String? avatarFileId;

  factory _$PrincipalRef([void Function(PrincipalRefBuilder)? updates]) =>
      (PrincipalRefBuilder()..update(updates))._build();

  _$PrincipalRef._({
    required this.id,
    required this.kind,
    required this.handle,
    required this.displayName,
    this.avatarFileId,
  }) : super._();
  @override
  PrincipalRef rebuild(void Function(PrincipalRefBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PrincipalRefBuilder toBuilder() => PrincipalRefBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PrincipalRef &&
        id == other.id &&
        kind == other.kind &&
        handle == other.handle &&
        displayName == other.displayName &&
        avatarFileId == other.avatarFileId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, avatarFileId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PrincipalRef')
          ..add('id', id)
          ..add('kind', kind)
          ..add('handle', handle)
          ..add('displayName', displayName)
          ..add('avatarFileId', avatarFileId))
        .toString();
  }
}

class PrincipalRefBuilder
    implements Builder<PrincipalRef, PrincipalRefBuilder> {
  _$PrincipalRef? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  PrincipalKind? _kind;
  PrincipalKind? get kind => _$this._kind;
  set kind(PrincipalKind? kind) => _$this._kind = kind;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _avatarFileId;
  String? get avatarFileId => _$this._avatarFileId;
  set avatarFileId(String? avatarFileId) => _$this._avatarFileId = avatarFileId;

  PrincipalRefBuilder() {
    PrincipalRef._defaults(this);
  }

  PrincipalRefBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _kind = $v.kind;
      _handle = $v.handle;
      _displayName = $v.displayName;
      _avatarFileId = $v.avatarFileId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PrincipalRef other) {
    _$v = other as _$PrincipalRef;
  }

  @override
  void update(void Function(PrincipalRefBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PrincipalRef build() => _build();

  _$PrincipalRef _build() {
    final _$result =
        _$v ??
        _$PrincipalRef._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'PrincipalRef', 'id'),
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'PrincipalRef',
            'kind',
          ),
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'PrincipalRef',
            'handle',
          ),
          displayName: BuiltValueNullFieldError.checkNotNull(
            displayName,
            r'PrincipalRef',
            'displayName',
          ),
          avatarFileId: avatarFileId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
