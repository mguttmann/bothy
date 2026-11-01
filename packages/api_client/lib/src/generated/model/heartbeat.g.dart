// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'heartbeat.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const HeartbeatStateEnum _$heartbeatStateEnum_online =
    const HeartbeatStateEnum._('online');
const HeartbeatStateEnum _$heartbeatStateEnum_degraded =
    const HeartbeatStateEnum._('degraded');
const HeartbeatStateEnum _$heartbeatStateEnum_unknownDefaultOpenApi =
    const HeartbeatStateEnum._('unknownDefaultOpenApi');

HeartbeatStateEnum _$heartbeatStateEnumValueOf(String name) {
  switch (name) {
    case 'online':
      return _$heartbeatStateEnum_online;
    case 'degraded':
      return _$heartbeatStateEnum_degraded;
    case 'unknownDefaultOpenApi':
      return _$heartbeatStateEnum_unknownDefaultOpenApi;
    default:
      return _$heartbeatStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<HeartbeatStateEnum> _$heartbeatStateEnumValues =
    BuiltSet<HeartbeatStateEnum>(const <HeartbeatStateEnum>[
      _$heartbeatStateEnum_online,
      _$heartbeatStateEnum_degraded,
      _$heartbeatStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<HeartbeatStateEnum> _$heartbeatStateEnumSerializer =
    _$HeartbeatStateEnumSerializer();

class _$HeartbeatStateEnumSerializer
    implements PrimitiveSerializer<HeartbeatStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'online': 'online',
    'degraded': 'degraded',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'online': 'online',
    'degraded': 'degraded',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[HeartbeatStateEnum];
  @override
  final String wireName = 'HeartbeatStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    HeartbeatStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  HeartbeatStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => HeartbeatStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Heartbeat extends Heartbeat {
  @override
  final HeartbeatStateEnum state;
  @override
  final String? runtimeVersion;
  @override
  final BillingMode? reportedBillingMode;
  @override
  final BuiltList<BillingMode>? billingModesSupported;
  @override
  final BillingEvidence? billingEvidence;
  @override
  final ToolClass? reportedToolClass;
  @override
  final AgentRateLimitState? rateLimit;
  @override
  final BuiltMap<String, JsonObject?>? details;
  @override
  final BuiltList<ProjectReadiness>? projectReadiness;
  @override
  final BuiltSet<String>? projectFolders;
  @override
  final BuiltList<ProjectTrashResult>? projectTrashResults;

  factory _$Heartbeat([void Function(HeartbeatBuilder)? updates]) =>
      (HeartbeatBuilder()..update(updates))._build();

  _$Heartbeat._({
    required this.state,
    this.runtimeVersion,
    this.reportedBillingMode,
    this.billingModesSupported,
    this.billingEvidence,
    this.reportedToolClass,
    this.rateLimit,
    this.details,
    this.projectReadiness,
    this.projectFolders,
    this.projectTrashResults,
  }) : super._();
  @override
  Heartbeat rebuild(void Function(HeartbeatBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  HeartbeatBuilder toBuilder() => HeartbeatBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Heartbeat &&
        state == other.state &&
        runtimeVersion == other.runtimeVersion &&
        reportedBillingMode == other.reportedBillingMode &&
        billingModesSupported == other.billingModesSupported &&
        billingEvidence == other.billingEvidence &&
        reportedToolClass == other.reportedToolClass &&
        rateLimit == other.rateLimit &&
        details == other.details &&
        projectReadiness == other.projectReadiness &&
        projectFolders == other.projectFolders &&
        projectTrashResults == other.projectTrashResults;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, runtimeVersion.hashCode);
    _$hash = $jc(_$hash, reportedBillingMode.hashCode);
    _$hash = $jc(_$hash, billingModesSupported.hashCode);
    _$hash = $jc(_$hash, billingEvidence.hashCode);
    _$hash = $jc(_$hash, reportedToolClass.hashCode);
    _$hash = $jc(_$hash, rateLimit.hashCode);
    _$hash = $jc(_$hash, details.hashCode);
    _$hash = $jc(_$hash, projectReadiness.hashCode);
    _$hash = $jc(_$hash, projectFolders.hashCode);
    _$hash = $jc(_$hash, projectTrashResults.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Heartbeat')
          ..add('state', state)
          ..add('runtimeVersion', runtimeVersion)
          ..add('reportedBillingMode', reportedBillingMode)
          ..add('billingModesSupported', billingModesSupported)
          ..add('billingEvidence', billingEvidence)
          ..add('reportedToolClass', reportedToolClass)
          ..add('rateLimit', rateLimit)
          ..add('details', details)
          ..add('projectReadiness', projectReadiness)
          ..add('projectFolders', projectFolders)
          ..add('projectTrashResults', projectTrashResults))
        .toString();
  }
}

class HeartbeatBuilder implements Builder<Heartbeat, HeartbeatBuilder> {
  _$Heartbeat? _$v;

  HeartbeatStateEnum? _state;
  HeartbeatStateEnum? get state => _$this._state;
  set state(HeartbeatStateEnum? state) => _$this._state = state;

  String? _runtimeVersion;
  String? get runtimeVersion => _$this._runtimeVersion;
  set runtimeVersion(String? runtimeVersion) =>
      _$this._runtimeVersion = runtimeVersion;

  BillingMode? _reportedBillingMode;
  BillingMode? get reportedBillingMode => _$this._reportedBillingMode;
  set reportedBillingMode(BillingMode? reportedBillingMode) =>
      _$this._reportedBillingMode = reportedBillingMode;

  ListBuilder<BillingMode>? _billingModesSupported;
  ListBuilder<BillingMode> get billingModesSupported =>
      _$this._billingModesSupported ??= ListBuilder<BillingMode>();
  set billingModesSupported(ListBuilder<BillingMode>? billingModesSupported) =>
      _$this._billingModesSupported = billingModesSupported;

  BillingEvidenceBuilder? _billingEvidence;
  BillingEvidenceBuilder get billingEvidence =>
      _$this._billingEvidence ??= BillingEvidenceBuilder();
  set billingEvidence(BillingEvidenceBuilder? billingEvidence) =>
      _$this._billingEvidence = billingEvidence;

  ToolClass? _reportedToolClass;
  ToolClass? get reportedToolClass => _$this._reportedToolClass;
  set reportedToolClass(ToolClass? reportedToolClass) =>
      _$this._reportedToolClass = reportedToolClass;

  AgentRateLimitStateBuilder? _rateLimit;
  AgentRateLimitStateBuilder get rateLimit =>
      _$this._rateLimit ??= AgentRateLimitStateBuilder();
  set rateLimit(AgentRateLimitStateBuilder? rateLimit) =>
      _$this._rateLimit = rateLimit;

  MapBuilder<String, JsonObject?>? _details;
  MapBuilder<String, JsonObject?> get details =>
      _$this._details ??= MapBuilder<String, JsonObject?>();
  set details(MapBuilder<String, JsonObject?>? details) =>
      _$this._details = details;

  ListBuilder<ProjectReadiness>? _projectReadiness;
  ListBuilder<ProjectReadiness> get projectReadiness =>
      _$this._projectReadiness ??= ListBuilder<ProjectReadiness>();
  set projectReadiness(ListBuilder<ProjectReadiness>? projectReadiness) =>
      _$this._projectReadiness = projectReadiness;

  SetBuilder<String>? _projectFolders;
  SetBuilder<String> get projectFolders =>
      _$this._projectFolders ??= SetBuilder<String>();
  set projectFolders(SetBuilder<String>? projectFolders) =>
      _$this._projectFolders = projectFolders;

  ListBuilder<ProjectTrashResult>? _projectTrashResults;
  ListBuilder<ProjectTrashResult> get projectTrashResults =>
      _$this._projectTrashResults ??= ListBuilder<ProjectTrashResult>();
  set projectTrashResults(
    ListBuilder<ProjectTrashResult>? projectTrashResults,
  ) => _$this._projectTrashResults = projectTrashResults;

  HeartbeatBuilder() {
    Heartbeat._defaults(this);
  }

  HeartbeatBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _runtimeVersion = $v.runtimeVersion;
      _reportedBillingMode = $v.reportedBillingMode;
      _billingModesSupported = $v.billingModesSupported?.toBuilder();
      _billingEvidence = $v.billingEvidence?.toBuilder();
      _reportedToolClass = $v.reportedToolClass;
      _rateLimit = $v.rateLimit?.toBuilder();
      _details = $v.details?.toBuilder();
      _projectReadiness = $v.projectReadiness?.toBuilder();
      _projectFolders = $v.projectFolders?.toBuilder();
      _projectTrashResults = $v.projectTrashResults?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Heartbeat other) {
    _$v = other as _$Heartbeat;
  }

  @override
  void update(void Function(HeartbeatBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Heartbeat build() => _build();

  _$Heartbeat _build() {
    _$Heartbeat _$result;
    try {
      _$result =
          _$v ??
          _$Heartbeat._(
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Heartbeat',
              'state',
            ),
            runtimeVersion: runtimeVersion,
            reportedBillingMode: reportedBillingMode,
            billingModesSupported: _billingModesSupported?.build(),
            billingEvidence: _billingEvidence?.build(),
            reportedToolClass: reportedToolClass,
            rateLimit: _rateLimit?.build(),
            details: _details?.build(),
            projectReadiness: _projectReadiness?.build(),
            projectFolders: _projectFolders?.build(),
            projectTrashResults: _projectTrashResults?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'billingModesSupported';
        _billingModesSupported?.build();
        _$failedField = 'billingEvidence';
        _billingEvidence?.build();

        _$failedField = 'rateLimit';
        _rateLimit?.build();
        _$failedField = 'details';
        _details?.build();
        _$failedField = 'projectReadiness';
        _projectReadiness?.build();
        _$failedField = 'projectFolders';
        _projectFolders?.build();
        _$failedField = 'projectTrashResults';
        _projectTrashResults?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Heartbeat',
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
