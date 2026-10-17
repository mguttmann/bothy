// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UserCreate extends UserCreate {
  @override
  final String handle;
  @override
  final String displayName;
  @override
  final String? email;
  @override
  final String? locale;
  @override
  final String? timeZone;
  @override
  final bool? localAccount;

  factory _$UserCreate([void Function(UserCreateBuilder)? updates]) =>
      (UserCreateBuilder()..update(updates))._build();

  _$UserCreate._({
    required this.handle,
    required this.displayName,
    this.email,
    this.locale,
    this.timeZone,
    this.localAccount,
  }) : super._();
  @override
  UserCreate rebuild(void Function(UserCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UserCreateBuilder toBuilder() => UserCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UserCreate &&
        handle == other.handle &&
        displayName == other.displayName &&
        email == other.email &&
        locale == other.locale &&
        timeZone == other.timeZone &&
        localAccount == other.localAccount;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, email.hashCode);
    _$hash = $jc(_$hash, locale.hashCode);
    _$hash = $jc(_$hash, timeZone.hashCode);
    _$hash = $jc(_$hash, localAccount.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UserCreate')
          ..add('handle', handle)
          ..add('displayName', displayName)
          ..add('email', email)
          ..add('locale', locale)
          ..add('timeZone', timeZone)
          ..add('localAccount', localAccount))
        .toString();
  }
}

class UserCreateBuilder implements Builder<UserCreate, UserCreateBuilder> {
  _$UserCreate? _$v;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  String? _displayName;
  String? get displayName => _$this._displayName;
  set displayName(String? displayName) => _$this._displayName = displayName;

  String? _email;
  String? get email => _$this._email;
  set email(String? email) => _$this._email = email;

  String? _locale;
  String? get locale => _$this._locale;
  set locale(String? locale) => _$this._locale = locale;

  String? _timeZone;
  String? get timeZone => _$this._timeZone;
  set timeZone(String? timeZone) => _$this._timeZone = timeZone;

  bool? _localAccount;
  bool? get localAccount => _$this._localAccount;
  set localAccount(bool? localAccount) => _$this._localAccount = localAccount;

  UserCreateBuilder() {
    UserCreate._defaults(this);
  }

  UserCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _handle = $v.handle;
      _displayName = $v.displayName;
      _email = $v.email;
      _locale = $v.locale;
      _timeZone = $v.timeZone;
      _localAccount = $v.localAccount;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UserCreate other) {
    _$v = other as _$UserCreate;
  }

  @override
  void update(void Function(UserCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UserCreate build() => _build();

  _$UserCreate _build() {
    final _$result =
        _$v ??
        _$UserCreate._(
          handle: BuiltValueNullFieldError.checkNotNull(
            handle,
            r'UserCreate',
            'handle',
          ),
          displayName: BuiltValueNullFieldError.checkNotNull(
            displayName,
            r'UserCreate',
            'displayName',
          ),
          email: email,
          locale: locale,
          timeZone: timeZone,
          localAccount: localAccount,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
