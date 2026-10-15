// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_session_user_assign.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostSessionUserAssign extends HostSessionUserAssign {
  @override
  final String userId;
  @override
  final String osUser;

  factory _$HostSessionUserAssign([
    void Function(HostSessionUserAssignBuilder)? updates,
  ]) => (HostSessionUserAssignBuilder()..update(updates))._build();

  _$HostSessionUserAssign._({required this.userId, required this.osUser})
    : super._();
  @override
  HostSessionUserAssign rebuild(
    void Function(HostSessionUserAssignBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostSessionUserAssignBuilder toBuilder() =>
      HostSessionUserAssignBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostSessionUserAssign &&
        userId == other.userId &&
        osUser == other.osUser;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, osUser.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostSessionUserAssign')
          ..add('userId', userId)
          ..add('osUser', osUser))
        .toString();
  }
}

class HostSessionUserAssignBuilder
    implements Builder<HostSessionUserAssign, HostSessionUserAssignBuilder> {
  _$HostSessionUserAssign? _$v;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _osUser;
  String? get osUser => _$this._osUser;
  set osUser(String? osUser) => _$this._osUser = osUser;

  HostSessionUserAssignBuilder() {
    HostSessionUserAssign._defaults(this);
  }

  HostSessionUserAssignBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _userId = $v.userId;
      _osUser = $v.osUser;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostSessionUserAssign other) {
    _$v = other as _$HostSessionUserAssign;
  }

  @override
  void update(void Function(HostSessionUserAssignBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostSessionUserAssign build() => _build();

  _$HostSessionUserAssign _build() {
    final _$result =
        _$v ??
        _$HostSessionUserAssign._(
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'HostSessionUserAssign',
            'userId',
          ),
          osUser: BuiltValueNullFieldError.checkNotNull(
            osUser,
            r'HostSessionUserAssign',
            'osUser',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
