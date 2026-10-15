// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'terminal_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$TerminalSession extends TerminalSession {
  @override
  final String id;
  @override
  final String hostId;
  @override
  final String userId;
  @override
  final String deviceId;
  @override
  final String profile;
  @override
  final TerminalSessionState state;
  @override
  final DateTime openedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? detachedAt;
  @override
  final DateTime? closedAt;
  @override
  final TerminalCloseReason? closeReason;
  @override
  final TerminalCloseDetail? closeDetail;
  @override
  final int? exitCode;
  @override
  final int bytesIn;
  @override
  final int bytesOut;
  @override
  final int version;

  factory _$TerminalSession([void Function(TerminalSessionBuilder)? updates]) =>
      (TerminalSessionBuilder()..update(updates))._build();

  _$TerminalSession._({
    required this.id,
    required this.hostId,
    required this.userId,
    required this.deviceId,
    required this.profile,
    required this.state,
    required this.openedAt,
    this.startedAt,
    this.detachedAt,
    this.closedAt,
    this.closeReason,
    this.closeDetail,
    this.exitCode,
    required this.bytesIn,
    required this.bytesOut,
    required this.version,
  }) : super._();
  @override
  TerminalSession rebuild(void Function(TerminalSessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  TerminalSessionBuilder toBuilder() => TerminalSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is TerminalSession &&
        id == other.id &&
        hostId == other.hostId &&
        userId == other.userId &&
        deviceId == other.deviceId &&
        profile == other.profile &&
        state == other.state &&
        openedAt == other.openedAt &&
        startedAt == other.startedAt &&
        detachedAt == other.detachedAt &&
        closedAt == other.closedAt &&
        closeReason == other.closeReason &&
        closeDetail == other.closeDetail &&
        exitCode == other.exitCode &&
        bytesIn == other.bytesIn &&
        bytesOut == other.bytesOut &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, deviceId.hashCode);
    _$hash = $jc(_$hash, profile.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, openedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, detachedAt.hashCode);
    _$hash = $jc(_$hash, closedAt.hashCode);
    _$hash = $jc(_$hash, closeReason.hashCode);
    _$hash = $jc(_$hash, closeDetail.hashCode);
    _$hash = $jc(_$hash, exitCode.hashCode);
    _$hash = $jc(_$hash, bytesIn.hashCode);
    _$hash = $jc(_$hash, bytesOut.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'TerminalSession')
          ..add('id', id)
          ..add('hostId', hostId)
          ..add('userId', userId)
          ..add('deviceId', deviceId)
          ..add('profile', profile)
          ..add('state', state)
          ..add('openedAt', openedAt)
          ..add('startedAt', startedAt)
          ..add('detachedAt', detachedAt)
          ..add('closedAt', closedAt)
          ..add('closeReason', closeReason)
          ..add('closeDetail', closeDetail)
          ..add('exitCode', exitCode)
          ..add('bytesIn', bytesIn)
          ..add('bytesOut', bytesOut)
          ..add('version', version))
        .toString();
  }
}

class TerminalSessionBuilder
    implements Builder<TerminalSession, TerminalSessionBuilder> {
  _$TerminalSession? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _deviceId;
  String? get deviceId => _$this._deviceId;
  set deviceId(String? deviceId) => _$this._deviceId = deviceId;

  String? _profile;
  String? get profile => _$this._profile;
  set profile(String? profile) => _$this._profile = profile;

  TerminalSessionState? _state;
  TerminalSessionState? get state => _$this._state;
  set state(TerminalSessionState? state) => _$this._state = state;

  DateTime? _openedAt;
  DateTime? get openedAt => _$this._openedAt;
  set openedAt(DateTime? openedAt) => _$this._openedAt = openedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _detachedAt;
  DateTime? get detachedAt => _$this._detachedAt;
  set detachedAt(DateTime? detachedAt) => _$this._detachedAt = detachedAt;

  DateTime? _closedAt;
  DateTime? get closedAt => _$this._closedAt;
  set closedAt(DateTime? closedAt) => _$this._closedAt = closedAt;

  TerminalCloseReason? _closeReason;
  TerminalCloseReason? get closeReason => _$this._closeReason;
  set closeReason(TerminalCloseReason? closeReason) =>
      _$this._closeReason = closeReason;

  TerminalCloseDetail? _closeDetail;
  TerminalCloseDetail? get closeDetail => _$this._closeDetail;
  set closeDetail(TerminalCloseDetail? closeDetail) =>
      _$this._closeDetail = closeDetail;

  int? _exitCode;
  int? get exitCode => _$this._exitCode;
  set exitCode(int? exitCode) => _$this._exitCode = exitCode;

  int? _bytesIn;
  int? get bytesIn => _$this._bytesIn;
  set bytesIn(int? bytesIn) => _$this._bytesIn = bytesIn;

  int? _bytesOut;
  int? get bytesOut => _$this._bytesOut;
  set bytesOut(int? bytesOut) => _$this._bytesOut = bytesOut;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  TerminalSessionBuilder() {
    TerminalSession._defaults(this);
  }

  TerminalSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _hostId = $v.hostId;
      _userId = $v.userId;
      _deviceId = $v.deviceId;
      _profile = $v.profile;
      _state = $v.state;
      _openedAt = $v.openedAt;
      _startedAt = $v.startedAt;
      _detachedAt = $v.detachedAt;
      _closedAt = $v.closedAt;
      _closeReason = $v.closeReason;
      _closeDetail = $v.closeDetail;
      _exitCode = $v.exitCode;
      _bytesIn = $v.bytesIn;
      _bytesOut = $v.bytesOut;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(TerminalSession other) {
    _$v = other as _$TerminalSession;
  }

  @override
  void update(void Function(TerminalSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  TerminalSession build() => _build();

  _$TerminalSession _build() {
    final _$result =
        _$v ??
        _$TerminalSession._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'TerminalSession',
            'id',
          ),
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'TerminalSession',
            'hostId',
          ),
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'TerminalSession',
            'userId',
          ),
          deviceId: BuiltValueNullFieldError.checkNotNull(
            deviceId,
            r'TerminalSession',
            'deviceId',
          ),
          profile: BuiltValueNullFieldError.checkNotNull(
            profile,
            r'TerminalSession',
            'profile',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'TerminalSession',
            'state',
          ),
          openedAt: BuiltValueNullFieldError.checkNotNull(
            openedAt,
            r'TerminalSession',
            'openedAt',
          ),
          startedAt: startedAt,
          detachedAt: detachedAt,
          closedAt: closedAt,
          closeReason: closeReason,
          closeDetail: closeDetail,
          exitCode: exitCode,
          bytesIn: BuiltValueNullFieldError.checkNotNull(
            bytesIn,
            r'TerminalSession',
            'bytesIn',
          ),
          bytesOut: BuiltValueNullFieldError.checkNotNull(
            bytesOut,
            r'TerminalSession',
            'bytesOut',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'TerminalSession',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
