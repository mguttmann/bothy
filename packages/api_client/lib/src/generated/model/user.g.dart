// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$User extends User {
  @override
  final String id;
  @override
  final String handle;
  @override
  final String displayName;
  @override
  final String? avatarFileId;
  @override
  final UserStatus status;
  @override
  final bool isOwner;
  @override
  final String? locale;
  @override
  final String? timeZone;
  @override
  final DateTime createdAt;

  factory _$User([void Function(UserBuilder)? updates]) =>
      (UserBuilder()..update(updates))._build();

  _$User._({
    required this.id,
    required this.handle,
    required this.displayName,
    this.avatarFileId,
    required this.status,
    required this.isOwner,
    this.locale,
    this.timeZone,
    required this.createdAt,
  }) : super._();
  @override
  User rebuild(void Function(UserBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserBuilder toBuilder() => UserBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is User &&
        id == other.id &&
        handle == other.handle &&
        displayName == other.displayName &&
        avatarFileId == other.avatarFileId &&
        status == other.status &&
        isOwner == other.isOwner &&
        locale == other.locale &&
        timeZone == other.timeZone &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, avatarFileId.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, isOwner.hashCode);
    _$hash = $jc(_$hash, locale.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'User')
          ..add('id', id)
          ..add('handle', handle)
          ..add('displayName', displayName)
          ..add('avatarFileId', avatarFileId)
          ..add('status', status)
          ..add('isOwner', isOwner)
          ..add('locale', locale)
          ..add('timeZone', timeZone)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class UserBuilder implements Builder<User, UserBuilder> {
  _$User? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _avatarFileId;
  String? get avatarFileId => _$this._avatarFileId;
  set avatarFileId(String? avatarFileId) => _$this._avatarFileId = avatarFileId;

  UserStatus? _status;
  UserStatus? get status => _$this._status;
  set status(UserStatus? status) => _$this._status = status;

  bool? _isOwner;
  bool? get isOwner => _$this._isOwner;
  set isOwner(bool? isOwner) => _$this._isOwner = isOwner;

  String? _locale;
  String? get locale => _$this._locale;
  set locale(String? locale) => _$this._locale = locale;

  String? _timeZone;
  String? get timeZone => _$this._timeZone;
  set timeZone(String? timeZone) => _$this._timeZone = timeZone;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  UserBuilder() {
    User._defaults(this);
  }

  UserBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _handle = $v.handle;
      _displayName = $v.displayName;
      _avatarFileId = $v.avatarFileId;
      _status = $v.status;
      _isOwner = $v.isOwner;
      _locale = $v.locale;
      _timeZone = $v.timeZone;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(User other) {
    _$v = other as _$User;
  }

  @override
  void update(void Function(UserBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  User build() => _build();

  _$User _build() {
    final _$result =
        _$v ??
        _$User._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'User', 'id'),
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'User',
            'handle',
          ),
          displayName: BuiltValueNullFieldError.checkNotNull(
            displayName,
            r'User',
            'displayName',
          ),
          avatarFileId: avatarFileId,
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'User',
            'status',
          ),
          isOwner: BuiltValueNullFieldError.checkNotNull(
            isOwner,
            r'User',
            'isOwner',
          ),
          locale: locale,
          timeZone: timeZone,
          createdAt: BuiltValueNullFieldError.checkNotNull(
            createdAt,
            r'User',
            'createdAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
