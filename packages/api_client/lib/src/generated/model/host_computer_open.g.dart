// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_computer_open.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostComputerOpen extends HostComputerOpen {
  @override
  final String sessionId;
  @override
  final String runId;
  @override
  final String agentId;
  @override
  final String agentName;
  @override
  final String userId;
  @override
  final String dataPath;
  @override
  final DateTime expiresAt;
  @override
  final HostComputerSettings settings;

  factory _$HostComputerOpen([
    void Function(HostComputerOpenBuilder)? updates,
  ]) => (HostComputerOpenBuilder()..update(updates))._build();

  _$HostComputerOpen._({
    required this.sessionId,
    required this.runId,
    required this.agentId,
    required this.agentName,
    required this.userId,
    required this.dataPath,
    required this.expiresAt,
    required this.settings,
  }) : super._();
  @override
  HostComputerOpen rebuild(void Function(HostComputerOpenBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HostComputerOpenBuilder toBuilder() =>
      HostComputerOpenBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostComputerOpen &&
        sessionId == other.sessionId &&
        runId == other.runId &&
        agentId == other.agentId &&
        agentName == other.agentName &&
        userId == other.userId &&
        dataPath == other.dataPath &&
        expiresAt == other.expiresAt &&
        settings == other.settings;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, sessionId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, agentName.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, dataPath.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, settings.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostComputerOpen')
          ..add('sessionId', sessionId)
          ..add('runId', runId)
          ..add('agentId', agentId)
          ..add('agentName', agentName)
          ..add('userId', userId)
          ..add('dataPath', dataPath)
          ..add('expiresAt', expiresAt)
          ..add('settings', settings))
        .toString();
  }
}

class HostComputerOpenBuilder
    implements Builder<HostComputerOpen, HostComputerOpenBuilder> {
  _$HostComputerOpen? _$v;

  String? _sessionId;
  String? get sessionId => _$this._sessionId;
  set sessionId(String? sessionId) => _$this._sessionId = sessionId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _agentName;
  String? get agentName => _$this._agentName;
  set agentName(String? agentName) => _$this._agentName = agentName;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _dataPath;
  String? get dataPath => _$this._dataPath;
  set dataPath(String? dataPath) => _$this._dataPath = dataPath;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  HostComputerSettingsBuilder? _settings;
  HostComputerSettingsBuilder get settings =>
      _$this._settings ??= HostComputerSettingsBuilder();
  set settings(HostComputerSettingsBuilder? settings) =>
      _$this._settings = settings;

  HostComputerOpenBuilder() {
    HostComputerOpen._defaults(this);
  }

  HostComputerOpenBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _sessionId = $v.sessionId;
      _runId = $v.runId;
      _agentId = $v.agentId;
      _agentName = $v.agentName;
      _userId = $v.userId;
      _dataPath = $v.dataPath;
      _expiresAt = $v.expiresAt;
      _settings = $v.settings.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostComputerOpen other) {
    _$v = other as _$HostComputerOpen;
  }

  @override
  void update(void Function(HostComputerOpenBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostComputerOpen build() => _build();

  _$HostComputerOpen _build() {
    _$HostComputerOpen _$result;
    try {
      _$result =
          _$v ??
          _$HostComputerOpen._(
            sessionId: BuiltValueNullFieldError.checkNotNull(
              sessionId,
              r'HostComputerOpen',
              'sessionId',
            ),
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'HostComputerOpen',
              'runId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'HostComputerOpen',
              'agentId',
            ),
            agentName: BuiltValueNullFieldError.checkNotNull(
              agentName,
              r'HostComputerOpen',
              'agentName',
            ),
            userId: BuiltValueNullFieldError.checkNotNull(
              userId,
              r'HostComputerOpen',
              'userId',
            ),
            dataPath: BuiltValueNullFieldError.checkNotNull(
              dataPath,
              r'HostComputerOpen',
              'dataPath',
            ),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'HostComputerOpen',
              'expiresAt',
            ),
            settings: settings.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'settings';
        settings.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostComputerOpen',
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
