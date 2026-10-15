// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostReport extends HostReport {
  @override
  final BuiltList<HostProfile> profiles;
  @override
  final BuiltList<HostReportedSessionUser> sessionUsers;
  @override
  final BuiltList<HostDoctorCheck> doctor;
  @override
  final bool isServerHost;
  @override
  final bool requireLocalConfirm;
  @override
  final String? osVersion;

  factory _$HostReport([void Function(HostReportBuilder)? updates]) =>
      (HostReportBuilder()..update(updates))._build();

  _$HostReport._({
    required this.profiles,
    required this.sessionUsers,
    required this.doctor,
    required this.isServerHost,
    required this.requireLocalConfirm,
    this.osVersion,
  }) : super._();
  @override
  HostReport rebuild(void Function(HostReportBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostReportBuilder toBuilder() => HostReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostReport &&
        profiles == other.profiles &&
        sessionUsers == other.sessionUsers &&
        doctor == other.doctor &&
        isServerHost == other.isServerHost &&
        requireLocalConfirm == other.requireLocalConfirm &&
        osVersion == other.osVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, profiles.hashCode);
    _$hash = $jc(_$hash, sessionUsers.hashCode);
    _$hash = $jc(_$hash, doctor.hashCode);
    _$hash = $jc(_$hash, isServerHost.hashCode);
    _$hash = $jc(_$hash, requireLocalConfirm.hashCode);
    _$hash = $jc(_$hash, osVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostReport')
          ..add('profiles', profiles)
          ..add('sessionUsers', sessionUsers)
          ..add('doctor', doctor)
          ..add('isServerHost', isServerHost)
          ..add('requireLocalConfirm', requireLocalConfirm)
          ..add('osVersion', osVersion))
        .toString();
  }
}

class HostReportBuilder implements Builder<HostReport, HostReportBuilder> {
  _$HostReport? _$v;

  ListBuilder<HostProfile>? _profiles;
  ListBuilder<HostProfile> get profiles =>
      _$this._profiles ??= ListBuilder<HostProfile>();
  set profiles(ListBuilder<HostProfile>? profiles) =>
      _$this._profiles = profiles;

  ListBuilder<HostReportedSessionUser>? _sessionUsers;
  ListBuilder<HostReportedSessionUser> get sessionUsers =>
      _$this._sessionUsers ??= ListBuilder<HostReportedSessionUser>();
  set sessionUsers(ListBuilder<HostReportedSessionUser>? sessionUsers) =>
      _$this._sessionUsers = sessionUsers;

  ListBuilder<HostDoctorCheck>? _doctor;
  ListBuilder<HostDoctorCheck> get doctor =>
      _$this._doctor ??= ListBuilder<HostDoctorCheck>();
  set doctor(ListBuilder<HostDoctorCheck>? doctor) => _$this._doctor = doctor;

  bool? _isServerHost;
  bool? get isServerHost => _$this._isServerHost;
  set isServerHost(bool? isServerHost) => _$this._isServerHost = isServerHost;

  bool? _requireLocalConfirm;
  bool? get requireLocalConfirm => _$this._requireLocalConfirm;
  set requireLocalConfirm(bool? requireLocalConfirm) =>
      _$this._requireLocalConfirm = requireLocalConfirm;

  String? _osVersion;
  String? get osVersion => _$this._osVersion;
  set osVersion(String? osVersion) => _$this._osVersion = osVersion;

  HostReportBuilder() {
    HostReport._defaults(this);
  }

  HostReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _profiles = $v.profiles.toBuilder();
      _sessionUsers = $v.sessionUsers.toBuilder();
      _doctor = $v.doctor.toBuilder();
      _isServerHost = $v.isServerHost;
      _requireLocalConfirm = $v.requireLocalConfirm;
      _osVersion = $v.osVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostReport other) {
    _$v = other as _$HostReport;
  }

  @override
  void update(void Function(HostReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostReport build() => _build();

  _$HostReport _build() {
    _$HostReport _$result;
    try {
      _$result =
          _$v ??
          _$HostReport._(
            profiles: profiles.build(),
            sessionUsers: sessionUsers.build(),
            doctor: doctor.build(),
            isServerHost: BuiltValueNullFieldError.checkNotNull(
              isServerHost,
              r'HostReport',
              'isServerHost',
            ),
            requireLocalConfirm: BuiltValueNullFieldError.checkNotNull(
              requireLocalConfirm,
              r'HostReport',
              'requireLocalConfirm',
            ),
            osVersion: osVersion,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'profiles';
        profiles.build();
        _$failedField = 'sessionUsers';
        sessionUsers.build();
        _$failedField = 'doctor';
        doctor.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostReport',
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
