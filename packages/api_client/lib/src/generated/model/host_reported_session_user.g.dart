// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_reported_session_user.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostReportedSessionUser extends HostReportedSessionUser {
  @override
  final String osUser;
  @override
  final BuiltList<String> workdirs;

  factory _$HostReportedSessionUser([
    void Function(HostReportedSessionUserBuilder)? updates,
  ]) => (HostReportedSessionUserBuilder()..update(updates))._build();

  _$HostReportedSessionUser._({required this.osUser, required this.workdirs})
    : super._();
  @override
  HostReportedSessionUser rebuild(
    void Function(HostReportedSessionUserBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostReportedSessionUserBuilder toBuilder() =>
      HostReportedSessionUserBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostReportedSessionUser &&
        osUser == other.osUser &&
        workdirs == other.workdirs;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, osUser.hashCode);
    _$hash = $jc(_$hash, workdirs.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostReportedSessionUser')
          ..add('osUser', osUser)
          ..add('workdirs', workdirs))
        .toString();
  }
}

class HostReportedSessionUserBuilder
    implements
        Builder<HostReportedSessionUser, HostReportedSessionUserBuilder> {
  _$HostReportedSessionUser? _$v;

  String? _osUser;
  String? get osUser => _$this._osUser;
  set osUser(String? osUser) => _$this._osUser = osUser;

  ListBuilder<String>? _workdirs;
  ListBuilder<String> get workdirs =>
      _$this._workdirs ??= ListBuilder<String>();
  set workdirs(ListBuilder<String>? workdirs) => _$this._workdirs = workdirs;

  HostReportedSessionUserBuilder() {
    HostReportedSessionUser._defaults(this);
  }

  HostReportedSessionUserBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _osUser = $v.osUser;
      _workdirs = $v.workdirs.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostReportedSessionUser other) {
    _$v = other as _$HostReportedSessionUser;
  }

  @override
  void update(void Function(HostReportedSessionUserBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostReportedSessionUser build() => _build();

  _$HostReportedSessionUser _build() {
    _$HostReportedSessionUser _$result;
    try {
      _$result =
          _$v ??
          _$HostReportedSessionUser._(
            osUser: BuiltValueNullFieldError.checkNotNull(
              osUser,
              r'HostReportedSessionUser',
              'osUser',
            ),
            workdirs: workdirs.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'workdirs';
        workdirs.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostReportedSessionUser',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
