// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'computer_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ComputerSessionPauseReasonEnum _$computerSessionPauseReasonEnum_locked =
    const ComputerSessionPauseReasonEnum._('locked');
const ComputerSessionPauseReasonEnum
_$computerSessionPauseReasonEnum_unknownDefaultOpenApi =
    const ComputerSessionPauseReasonEnum._('unknownDefaultOpenApi');

ComputerSessionPauseReasonEnum _$computerSessionPauseReasonEnumValueOf(
  String name,
) {
  switch (name) {
    case 'locked':
      return _$computerSessionPauseReasonEnum_locked;
    case 'unknownDefaultOpenApi':
      return _$computerSessionPauseReasonEnum_unknownDefaultOpenApi;
    default:
      return _$computerSessionPauseReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ComputerSessionPauseReasonEnum>
_$computerSessionPauseReasonEnumValues =
    BuiltSet<ComputerSessionPauseReasonEnum>(
      const <ComputerSessionPauseReasonEnum>[
        _$computerSessionPauseReasonEnum_locked,
        _$computerSessionPauseReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<ComputerSessionPauseReasonEnum>
_$computerSessionPauseReasonEnumSerializer =
    _$ComputerSessionPauseReasonEnumSerializer();

class _$ComputerSessionPauseReasonEnumSerializer
    implements PrimitiveSerializer<ComputerSessionPauseReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'locked': 'locked',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'locked': 'locked',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ComputerSessionPauseReasonEnum];
  @override
  final String wireName = 'ComputerSessionPauseReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    ComputerSessionPauseReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ComputerSessionPauseReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ComputerSessionPauseReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ComputerSession extends ComputerSession {
  @override
  final String id;
  @override
  final String hostId;
  @override
  final String desktop;
  @override
  final String userId;
  @override
  final String agentId;
  @override
  final String runId;
  @override
  final ComputerSessionState state;
  @override
  final ComputerSessionPauseReasonEnum? pauseReason;
  @override
  final int generation;
  @override
  final int keyGeneration;
  @override
  final DateTime? attachedAt;
  @override
  final DateTime openedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? pausedAt;
  @override
  final int pausedMs;
  @override
  final DateTime? closedAt;
  @override
  final String? stoppedBy;
  @override
  final ComputerCloseReason? closeReason;
  @override
  final ComputerCloseDetail? closeDetail;
  @override
  final int actionCount;
  @override
  final String? frontApp;
  @override
  final int version;

  factory _$ComputerSession([void Function(ComputerSessionBuilder)? updates]) =>
      (ComputerSessionBuilder()..update(updates))._build();

  _$ComputerSession._({
    required this.id,
    required this.hostId,
    required this.desktop,
    required this.userId,
    required this.agentId,
    required this.runId,
    required this.state,
    this.pauseReason,
    required this.generation,
    required this.keyGeneration,
    this.attachedAt,
    required this.openedAt,
    this.startedAt,
    this.pausedAt,
    required this.pausedMs,
    this.closedAt,
    this.stoppedBy,
    this.closeReason,
    this.closeDetail,
    required this.actionCount,
    this.frontApp,
    required this.version,
  }) : super._();
  @override
  ComputerSession rebuild(void Function(ComputerSessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ComputerSessionBuilder toBuilder() => ComputerSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ComputerSession &&
        id == other.id &&
        hostId == other.hostId &&
        desktop == other.desktop &&
        userId == other.userId &&
        agentId == other.agentId &&
        runId == other.runId &&
        state == other.state &&
        pauseReason == other.pauseReason &&
        generation == other.generation &&
        keyGeneration == other.keyGeneration &&
        attachedAt == other.attachedAt &&
        openedAt == other.openedAt &&
        startedAt == other.startedAt &&
        pausedAt == other.pausedAt &&
        pausedMs == other.pausedMs &&
        closedAt == other.closedAt &&
        stoppedBy == other.stoppedBy &&
        closeReason == other.closeReason &&
        closeDetail == other.closeDetail &&
        actionCount == other.actionCount &&
        frontApp == other.frontApp &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, desktop.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, pauseReason.hashCode);
    _$hash = $jc(_$hash, generation.hashCode);
    _$hash = $jc(_$hash, keyGeneration.hashCode);
    _$hash = $jc(_$hash, attachedAt.hashCode);
    _$hash = $jc(_$hash, openedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, pausedAt.hashCode);
    _$hash = $jc(_$hash, pausedMs.hashCode);
    _$hash = $jc(_$hash, closedAt.hashCode);
    _$hash = $jc(_$hash, stoppedBy.hashCode);
    _$hash = $jc(_$hash, closeReason.hashCode);
    _$hash = $jc(_$hash, closeDetail.hashCode);
    _$hash = $jc(_$hash, actionCount.hashCode);
    _$hash = $jc(_$hash, frontApp.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ComputerSession')
          ..add('id', id)
          ..add('hostId', hostId)
          ..add('desktop', desktop)
          ..add('userId', userId)
          ..add('agentId', agentId)
          ..add('runId', runId)
          ..add('state', state)
          ..add('pauseReason', pauseReason)
          ..add('generation', generation)
          ..add('keyGeneration', keyGeneration)
          ..add('attachedAt', attachedAt)
          ..add('openedAt', openedAt)
          ..add('startedAt', startedAt)
          ..add('pausedAt', pausedAt)
          ..add('pausedMs', pausedMs)
          ..add('closedAt', closedAt)
          ..add('stoppedBy', stoppedBy)
          ..add('closeReason', closeReason)
          ..add('closeDetail', closeDetail)
          ..add('actionCount', actionCount)
          ..add('frontApp', frontApp)
          ..add('version', version))
        .toString();
  }
}

class ComputerSessionBuilder
    implements Builder<ComputerSession, ComputerSessionBuilder> {
  _$ComputerSession? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _desktop;
  String? get desktop => _$this._desktop;
  set desktop(String? desktop) => _$this._desktop = desktop;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  ComputerSessionState? _state;
  ComputerSessionState? get state => _$this._state;
  set state(ComputerSessionState? state) => _$this._state = state;

  ComputerSessionPauseReasonEnum? _pauseReason;
  ComputerSessionPauseReasonEnum? get pauseReason => _$this._pauseReason;
  set pauseReason(ComputerSessionPauseReasonEnum? pauseReason) =>
      _$this._pauseReason = pauseReason;

  int? _generation;
  int? get generation => _$this._generation;
  set generation(int? generation) => _$this._generation = generation;

  int? _keyGeneration;
  int? get keyGeneration => _$this._keyGeneration;
  set keyGeneration(int? keyGeneration) =>
      _$this._keyGeneration = keyGeneration;

  DateTime? _attachedAt;
  DateTime? get attachedAt => _$this._attachedAt;
  set attachedAt(DateTime? attachedAt) => _$this._attachedAt = attachedAt;

  DateTime? _openedAt;
  DateTime? get openedAt => _$this._openedAt;
  set openedAt(DateTime? openedAt) => _$this._openedAt = openedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _pausedAt;
  DateTime? get pausedAt => _$this._pausedAt;
  set pausedAt(DateTime? pausedAt) => _$this._pausedAt = pausedAt;

  int? _pausedMs;
  int? get pausedMs => _$this._pausedMs;
  set pausedMs(int? pausedMs) => _$this._pausedMs = pausedMs;

  DateTime? _closedAt;
  DateTime? get closedAt => _$this._closedAt;
  set closedAt(DateTime? closedAt) => _$this._closedAt = closedAt;

  String? _stoppedBy;
  String? get stoppedBy => _$this._stoppedBy;
  set stoppedBy(String? stoppedBy) => _$this._stoppedBy = stoppedBy;

  ComputerCloseReason? _closeReason;
  ComputerCloseReason? get closeReason => _$this._closeReason;
  set closeReason(ComputerCloseReason? closeReason) =>
      _$this._closeReason = closeReason;

  ComputerCloseDetail? _closeDetail;
  ComputerCloseDetail? get closeDetail => _$this._closeDetail;
  set closeDetail(ComputerCloseDetail? closeDetail) =>
      _$this._closeDetail = closeDetail;

  int? _actionCount;
  int? get actionCount => _$this._actionCount;
  set actionCount(int? actionCount) => _$this._actionCount = actionCount;

  String? _frontApp;
  String? get frontApp => _$this._frontApp;
  set frontApp(String? frontApp) => _$this._frontApp = frontApp;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  ComputerSessionBuilder() {
    ComputerSession._defaults(this);
  }

  ComputerSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _hostId = $v.hostId;
      _desktop = $v.desktop;
      _userId = $v.userId;
      _agentId = $v.agentId;
      _runId = $v.runId;
      _state = $v.state;
      _pauseReason = $v.pauseReason;
      _generation = $v.generation;
      _keyGeneration = $v.keyGeneration;
      _attachedAt = $v.attachedAt;
      _openedAt = $v.openedAt;
      _startedAt = $v.startedAt;
      _pausedAt = $v.pausedAt;
      _pausedMs = $v.pausedMs;
      _closedAt = $v.closedAt;
      _stoppedBy = $v.stoppedBy;
      _closeReason = $v.closeReason;
      _closeDetail = $v.closeDetail;
      _actionCount = $v.actionCount;
      _frontApp = $v.frontApp;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ComputerSession other) {
    _$v = other as _$ComputerSession;
  }

  @override
  void update(void Function(ComputerSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ComputerSession build() => _build();

  _$ComputerSession _build() {
    final _$result =
        _$v ??
        _$ComputerSession._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'ComputerSession',
            'id',
          ),
          hostId: BuiltValueNullFieldError.checkNotNull(
            hostId,
            r'ComputerSession',
            'hostId',
          ),
          desktop: BuiltValueNullFieldError.checkNotNull(
            desktop,
            r'ComputerSession',
            'desktop',
          ),
          userId: BuiltValueNullFieldError.checkNotNull(
            userId,
            r'ComputerSession',
            'userId',
          ),
          agentId: BuiltValueNullFieldError.checkNotNull(
            agentId,
            r'ComputerSession',
            'agentId',
          ),
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'ComputerSession',
            'runId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'ComputerSession',
            'state',
          ),
          pauseReason: pauseReason,
          generation: BuiltValueNullFieldError.checkNotNull(
            generation,
            r'ComputerSession',
            'generation',
          ),
          keyGeneration: BuiltValueNullFieldError.checkNotNull(
            keyGeneration,
            r'ComputerSession',
            'keyGeneration',
          ),
          attachedAt: attachedAt,
          openedAt: BuiltValueNullFieldError.checkNotNull(
            openedAt,
            r'ComputerSession',
            'openedAt',
          ),
          startedAt: startedAt,
          pausedAt: pausedAt,
          pausedMs: BuiltValueNullFieldError.checkNotNull(
            pausedMs,
            r'ComputerSession',
            'pausedMs',
          ),
          closedAt: closedAt,
          stoppedBy: stoppedBy,
          closeReason: closeReason,
          closeDetail: closeDetail,
          actionCount: BuiltValueNullFieldError.checkNotNull(
            actionCount,
            r'ComputerSession',
            'actionCount',
          ),
          frontApp: frontApp,
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'ComputerSession',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
