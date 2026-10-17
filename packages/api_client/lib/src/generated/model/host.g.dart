// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HostDisabledReasonEnum _$hostDisabledReasonEnum_owner =
    const HostDisabledReasonEnum._('owner');
const HostDisabledReasonEnum _$hostDisabledReasonEnum_duplicateConnection =
    const HostDisabledReasonEnum._('duplicateConnection');
const HostDisabledReasonEnum _$hostDisabledReasonEnum_unknownDefaultOpenApi =
    const HostDisabledReasonEnum._('unknownDefaultOpenApi');

HostDisabledReasonEnum _$hostDisabledReasonEnumValueOf(String name) {
  switch (name) {
    case 'owner':
      return _$hostDisabledReasonEnum_owner;
    case 'duplicateConnection':
      return _$hostDisabledReasonEnum_duplicateConnection;
    case 'unknownDefaultOpenApi':
      return _$hostDisabledReasonEnum_unknownDefaultOpenApi;
    default:
      return _$hostDisabledReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HostDisabledReasonEnum> _$hostDisabledReasonEnumValues =
    BuiltSet<HostDisabledReasonEnum>(const <HostDisabledReasonEnum>[
      _$hostDisabledReasonEnum_owner,
      _$hostDisabledReasonEnum_duplicateConnection,
      _$hostDisabledReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<HostDisabledReasonEnum> _$hostDisabledReasonEnumSerializer =
    _$HostDisabledReasonEnumSerializer();

class _$HostDisabledReasonEnumSerializer
    implements PrimitiveSerializer<HostDisabledReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'owner': 'owner',
    'duplicateConnection': 'duplicate_connection',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'owner': 'owner',
    'duplicate_connection': 'duplicateConnection',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HostDisabledReasonEnum];
  @override
  final String wireName = 'HostDisabledReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    HostDisabledReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HostDisabledReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HostDisabledReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Host extends Host {
  @override
  final String id;
  @override
  final String name;
  @override
  final HostState state;
  @override
  final HostDisabledReasonEnum? disabledReason;
  @override
  final HostPlatform? platform;
  @override
  final String? assignedUserId;
  @override
  final LlmCredentials llmCredentials;
  @override
  final String? credentialHolderId;
  @override
  final bool declaredServerHost;
  @override
  final bool? reportedIsServerHost;
  @override
  final String? noisePublicKey;
  @override
  final String? keyFingerprint;
  @override
  final int keyGeneration;
  @override
  final BuiltList<HostProfile> profiles;
  @override
  final BuiltList<HostDoctorCheck> doctor;
  @override
  final BuiltList<HostSessionUser>? sessionUsers;
  @override
  final BuiltList<HostReportedSessionUser>? reportedSessionUsers;
  @override
  final bool? requireLocalConfirm;
  @override
  final bool connected;
  @override
  final String? hostVersion;
  @override
  final String? osVersion;
  @override
  final DateTime? lastSeenAt;
  @override
  final DateTime createdAt;
  @override
  final int version;

  factory _$Host([void Function(HostBuilder)? updates]) =>
      (HostBuilder()..update(updates))._build();

  _$Host._({
    required this.id,
    required this.name,
    required this.state,
    this.disabledReason,
    this.platform,
    this.assignedUserId,
    required this.llmCredentials,
    this.credentialHolderId,
    required this.declaredServerHost,
    this.reportedIsServerHost,
    this.noisePublicKey,
    this.keyFingerprint,
    required this.keyGeneration,
    required this.profiles,
    required this.doctor,
    this.sessionUsers,
    this.reportedSessionUsers,
    this.requireLocalConfirm,
    required this.connected,
    this.hostVersion,
    this.osVersion,
    this.lastSeenAt,
    required this.createdAt,
    required this.version,
  }) : super._();
  @override
  Host rebuild(void Function(HostBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostBuilder toBuilder() => HostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Host &&
        id == other.id &&
        name == other.name &&
        state == other.state &&
        disabledReason == other.disabledReason &&
        platform == other.platform &&
        assignedUserId == other.assignedUserId &&
        llmCredentials == other.llmCredentials &&
        credentialHolderId == other.credentialHolderId &&
        declaredServerHost == other.declaredServerHost &&
        reportedIsServerHost == other.reportedIsServerHost &&
        noisePublicKey == other.noisePublicKey &&
        keyFingerprint == other.keyFingerprint &&
        keyGeneration == other.keyGeneration &&
        profiles == other.profiles &&
        doctor == other.doctor &&
        sessionUsers == other.sessionUsers &&
        reportedSessionUsers == other.reportedSessionUsers &&
        requireLocalConfirm == other.requireLocalConfirm &&
        connected == other.connected &&
        hostVersion == other.hostVersion &&
        osVersion == other.osVersion &&
        lastSeenAt == other.lastSeenAt &&
        createdAt == other.createdAt &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, disabledReason.hashCode);
    _$hash = $jc(_$hash, platform.hashCode);
    _$hash = $jc(_$hash, assignedUserId.hashCode);
    _$hash = $jc(_$hash, llmCredentials.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, declaredServerHost.hashCode);
    _$hash = $jc(_$hash, reportedIsServerHost.hashCode);
    _$hash = $jc(_$hash, noisePublicKey.hashCode);
    _$hash = $jc(_$hash, keyFingerprint.hashCode);
    _$hash = $jc(_$hash, keyGeneration.hashCode);
    _$hash = $jc(_$hash, profiles.hashCode);
    _$hash = $jc(_$hash, doctor.hashCode);
    _$hash = $jc(_$hash, sessionUsers.hashCode);
    _$hash = $jc(_$hash, reportedSessionUsers.hashCode);
    _$hash = $jc(_$hash, requireLocalConfirm.hashCode);
    _$hash = $jc(_$hash, connected.hashCode);
    _$hash = $jc(_$hash, hostVersion.hashCode);
    _$hash = $jc(_$hash, osVersion.hashCode);
    _$hash = $jc(_$hash, lastSeenAt.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Host')
          ..add('id', id)
          ..add('name', name)
          ..add('state', state)
          ..add('disabledReason', disabledReason)
          ..add('platform', platform)
          ..add('assignedUserId', assignedUserId)
          ..add('llmCredentials', llmCredentials)
          ..add('credentialHolderId', credentialHolderId)
          ..add('declaredServerHost', declaredServerHost)
          ..add('reportedIsServerHost', reportedIsServerHost)
          ..add('noisePublicKey', noisePublicKey)
          ..add('keyFingerprint', keyFingerprint)
          ..add('keyGeneration', keyGeneration)
          ..add('profiles', profiles)
          ..add('doctor', doctor)
          ..add('sessionUsers', sessionUsers)
          ..add('reportedSessionUsers', reportedSessionUsers)
          ..add('requireLocalConfirm', requireLocalConfirm)
          ..add('connected', connected)
          ..add('hostVersion', hostVersion)
          ..add('osVersion', osVersion)
          ..add('lastSeenAt', lastSeenAt)
          ..add('createdAt', createdAt)
          ..add('version', version))
        .toString();
  }
}

class HostBuilder implements Builder<Host, HostBuilder> {
  _$Host? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  HostState? _state;
  HostState? get state => _$this._state;
  set state(HostState? state) => _$this._state = state;

  HostDisabledReasonEnum? _disabledReason;
  HostDisabledReasonEnum? get disabledReason => _$this._disabledReason;
  set disabledReason(HostDisabledReasonEnum? disabledReason) =>
      _$this._disabledReason = disabledReason;

  HostPlatform? _platform;
  HostPlatform? get platform => _$this._platform;
  set platform(HostPlatform? platform) => _$this._platform = platform;

  String? _assignedUserId;
  String? get assignedUserId => _$this._assignedUserId;
  set assignedUserId(String? assignedUserId) =>
      _$this._assignedUserId = assignedUserId;

  LlmCredentials? _llmCredentials;
  LlmCredentials? get llmCredentials => _$this._llmCredentials;
  set llmCredentials(LlmCredentials? llmCredentials) =>
      _$this._llmCredentials = llmCredentials;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  bool? _declaredServerHost;
  bool? get declaredServerHost => _$this._declaredServerHost;
  set declaredServerHost(bool? declaredServerHost) =>
      _$this._declaredServerHost = declaredServerHost;

  bool? _reportedIsServerHost;
  bool? get reportedIsServerHost => _$this._reportedIsServerHost;
  set reportedIsServerHost(bool? reportedIsServerHost) =>
      _$this._reportedIsServerHost = reportedIsServerHost;

  String? _noisePublicKey;
  String? get noisePublicKey => _$this._noisePublicKey;
  set noisePublicKey(String? noisePublicKey) =>
      _$this._noisePublicKey = noisePublicKey;

  String? _keyFingerprint;
  String? get keyFingerprint => _$this._keyFingerprint;
  set keyFingerprint(String? keyFingerprint) =>
      _$this._keyFingerprint = keyFingerprint;

  int? _keyGeneration;
  int? get keyGeneration => _$this._keyGeneration;
  set keyGeneration(int? keyGeneration) =>
      _$this._keyGeneration = keyGeneration;

  ListBuilder<HostProfile>? _profiles;
  ListBuilder<HostProfile> get profiles =>
      _$this._profiles ??= ListBuilder<HostProfile>();
  set profiles(ListBuilder<HostProfile>? profiles) =>
      _$this._profiles = profiles;

  ListBuilder<HostDoctorCheck>? _doctor;
  ListBuilder<HostDoctorCheck> get doctor =>
      _$this._doctor ??= ListBuilder<HostDoctorCheck>();
  set doctor(ListBuilder<HostDoctorCheck>? doctor) => _$this._doctor = doctor;

  ListBuilder<HostSessionUser>? _sessionUsers;
  ListBuilder<HostSessionUser> get sessionUsers =>
      _$this._sessionUsers ??= ListBuilder<HostSessionUser>();
  set sessionUsers(ListBuilder<HostSessionUser>? sessionUsers) =>
      _$this._sessionUsers = sessionUsers;

  ListBuilder<HostReportedSessionUser>? _reportedSessionUsers;
  ListBuilder<HostReportedSessionUser> get reportedSessionUsers =>
      _$this._reportedSessionUsers ??= ListBuilder<HostReportedSessionUser>();
  set reportedSessionUsers(
    ListBuilder<HostReportedSessionUser>? reportedSessionUsers,
  ) => _$this._reportedSessionUsers = reportedSessionUsers;

  bool? _requireLocalConfirm;
  bool? get requireLocalConfirm => _$this._requireLocalConfirm;
  set requireLocalConfirm(bool? requireLocalConfirm) =>
      _$this._requireLocalConfirm = requireLocalConfirm;

  bool? _connected;
  bool? get connected => _$this._connected;
  set connected(bool? connected) => _$this._connected = connected;

  String? _hostVersion;
  String? get hostVersion => _$this._hostVersion;
  set hostVersion(String? hostVersion) => _$this._hostVersion = hostVersion;

  String? _osVersion;
  String? get osVersion => _$this._osVersion;
  set osVersion(String? osVersion) => _$this._osVersion = osVersion;

  DateTime? _lastSeenAt;
  DateTime? get lastSeenAt => _$this._lastSeenAt;
  set lastSeenAt(DateTime? lastSeenAt) => _$this._lastSeenAt = lastSeenAt;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  HostBuilder() {
    Host._defaults(this);
  }

  HostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _name = $v.name;
      _state = $v.state;
      _disabledReason = $v.disabledReason;
      _platform = $v.platform;
      _assignedUserId = $v.assignedUserId;
      _llmCredentials = $v.llmCredentials;
      _credentialHolderId = $v.credentialHolderId;
      _declaredServerHost = $v.declaredServerHost;
      _reportedIsServerHost = $v.reportedIsServerHost;
      _noisePublicKey = $v.noisePublicKey;
      _keyFingerprint = $v.keyFingerprint;
      _keyGeneration = $v.keyGeneration;
      _profiles = $v.profiles.toBuilder();
      _doctor = $v.doctor.toBuilder();
      _sessionUsers = $v.sessionUsers?.toBuilder();
      _reportedSessionUsers = $v.reportedSessionUsers?.toBuilder();
      _requireLocalConfirm = $v.requireLocalConfirm;
      _connected = $v.connected;
      _hostVersion = $v.hostVersion;
      _osVersion = $v.osVersion;
      _lastSeenAt = $v.lastSeenAt;
      _createdAt = $v.createdAt;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Host other) {
    _$v = other as _$Host;
  }

  @override
  void update(void Function(HostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Host build() => _build();

  _$Host _build() {
    _$Host _$result;
    try {
      _$result =
          _$v ??
          _$Host._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Host', 'id'),
            name: BuiltValueNullFieldError.checkNotNull(name, r'Host', 'name'),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Host',
              'state',
            ),
            disabledReason: disabledReason,
            platform: platform,
            assignedUserId: assignedUserId,
            llmCredentials: BuiltValueNullFieldError.checkNotNull(
              llmCredentials,
              r'Host',
              'llmCredentials',
            ),
            credentialHolderId: credentialHolderId,
            declaredServerHost: BuiltValueNullFieldError.checkNotNull(
              declaredServerHost,
              r'Host',
              'declaredServerHost',
            ),
            reportedIsServerHost: reportedIsServerHost,
            noisePublicKey: noisePublicKey,
            keyFingerprint: keyFingerprint,
            keyGeneration: BuiltValueNullFieldError.checkNotNull(
              keyGeneration,
              r'Host',
              'keyGeneration',
            ),
            profiles: profiles.build(),
            doctor: doctor.build(),
            sessionUsers: _sessionUsers?.build(),
            reportedSessionUsers: _reportedSessionUsers?.build(),
            requireLocalConfirm: requireLocalConfirm,
            connected: BuiltValueNullFieldError.checkNotNull(
              connected,
              r'Host',
              'connected',
            ),
            hostVersion: hostVersion,
            osVersion: osVersion,
            lastSeenAt: lastSeenAt,
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Host',
              'createdAt',
            ),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Host',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'profiles';
        profiles.build();
        _$failedField = 'doctor';
        doctor.build();
        _$failedField = 'sessionUsers';
        _sessionUsers?.build();
        _$failedField = 'reportedSessionUsers';
        _reportedSessionUsers?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Host', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
