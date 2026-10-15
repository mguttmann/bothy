// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentStatusEnum _$agentStatusEnum_active = const AgentStatusEnum._(
  'active',
);
const AgentStatusEnum _$agentStatusEnum_disabled = const AgentStatusEnum._(
  'disabled',
);
const AgentStatusEnum _$agentStatusEnum_deleted = const AgentStatusEnum._(
  'deleted',
);
const AgentStatusEnum _$agentStatusEnum_unknownDefaultOpenApi =
    const AgentStatusEnum._('unknownDefaultOpenApi');

AgentStatusEnum _$agentStatusEnumValueOf(String name) {
  switch (name) {
    case 'active':
      return _$agentStatusEnum_active;
    case 'disabled':
      return _$agentStatusEnum_disabled;
    case 'deleted':
      return _$agentStatusEnum_deleted;
    case 'unknownDefaultOpenApi':
      return _$agentStatusEnum_unknownDefaultOpenApi;
    default:
      return _$agentStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentStatusEnum> _$agentStatusEnumValues =
    BuiltSet<AgentStatusEnum>(const <AgentStatusEnum>[
      _$agentStatusEnum_active,
      _$agentStatusEnum_disabled,
      _$agentStatusEnum_deleted,
      _$agentStatusEnum_unknownDefaultOpenApi,
    ]);

Serializer<AgentStatusEnum> _$agentStatusEnumSerializer =
    _$AgentStatusEnumSerializer();

class _$AgentStatusEnumSerializer
    implements PrimitiveSerializer<AgentStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'active': 'active',
    'disabled': 'disabled',
    'deleted': 'deleted',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'active': 'active',
    'disabled': 'disabled',
    'deleted': 'deleted',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentStatusEnum];
  @override
  final String wireName = 'AgentStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Agent extends Agent {
  @override
  final String id;
  @override
  final String handle;
  @override
  final String displayName;
  @override
  final String? avatarFileId;
  @override
  final AgentStatusEnum status;
  @override
  final String adapterKind;
  @override
  final String description;
  @override
  final BillingMode billingMode;
  @override
  final String? credentialHolderId;
  @override
  final DateTime? billingModeChangedAt;
  @override
  final ToolClass toolClass;
  @override
  final AgentRuntimeStatus presence;
  @override
  final BuiltMap<String, JsonObject?> operatorConfig;
  @override
  final int version;

  factory _$Agent([void Function(AgentBuilder)? updates]) =>
      (AgentBuilder()..update(updates))._build();

  _$Agent._({
    required this.id,
    required this.handle,
    required this.displayName,
    this.avatarFileId,
    required this.status,
    required this.adapterKind,
    required this.description,
    required this.billingMode,
    this.credentialHolderId,
    this.billingModeChangedAt,
    required this.toolClass,
    required this.presence,
    required this.operatorConfig,
    required this.version,
  }) : super._();
  @override
  Agent rebuild(void Function(AgentBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentBuilder toBuilder() => AgentBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Agent &&
        id == other.id &&
        handle == other.handle &&
        displayName == other.displayName &&
        avatarFileId == other.avatarFileId &&
        status == other.status &&
        adapterKind == other.adapterKind &&
        description == other.description &&
        billingMode == other.billingMode &&
        credentialHolderId == other.credentialHolderId &&
        billingModeChangedAt == other.billingModeChangedAt &&
        toolClass == other.toolClass &&
        presence == other.presence &&
        operatorConfig == other.operatorConfig &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, displayName.hashCode);
    _$hash = $jc(_$hash, avatarFileId.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, adapterKind.hashCode);
    _$hash = $jc(_$hash, description.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, billingModeChangedAt.hashCode);
    _$hash = $jc(_$hash, toolClass.hashCode);
    _$hash = $jc(_$hash, presence.hashCode);
    _$hash = $jc(_$hash, operatorConfig.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Agent')
          ..add('id', id)
          ..add('handle', handle)
          ..add('displayName', displayName)
          ..add('avatarFileId', avatarFileId)
          ..add('status', status)
          ..add('adapterKind', adapterKind)
          ..add('description', description)
          ..add('billingMode', billingMode)
          ..add('credentialHolderId', credentialHolderId)
          ..add('billingModeChangedAt', billingModeChangedAt)
          ..add('toolClass', toolClass)
          ..add('presence', presence)
          ..add('operatorConfig', operatorConfig)
          ..add('version', version))
        .toString();
  }
}

class AgentBuilder implements Builder<Agent, AgentBuilder> {
  _$Agent? _$v;

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

  AgentStatusEnum? _status;
  AgentStatusEnum? get status => _$this._status;
  set status(AgentStatusEnum? status) => _$this._status = status;

  String? _adapterKind;
  String? get adapterKind => _$this._adapterKind;
  set adapterKind(String? adapterKind) => _$this._adapterKind = adapterKind;

  String? _description;
  String? get description => _$this._description;
  set description(String? description) => _$this._description = description;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  DateTime? _billingModeChangedAt;
  DateTime? get billingModeChangedAt => _$this._billingModeChangedAt;
  set billingModeChangedAt(DateTime? billingModeChangedAt) =>
      _$this._billingModeChangedAt = billingModeChangedAt;

  ToolClass? _toolClass;
  ToolClass? get toolClass => _$this._toolClass;
  set toolClass(ToolClass? toolClass) => _$this._toolClass = toolClass;

  AgentRuntimeStatusBuilder? _presence;
  AgentRuntimeStatusBuilder get presence =>
      _$this._presence ??= AgentRuntimeStatusBuilder();
  set presence(AgentRuntimeStatusBuilder? presence) =>
      _$this._presence = presence;

  MapBuilder<String, JsonObject?>? _operatorConfig;
  MapBuilder<String, JsonObject?> get operatorConfig =>
      _$this._operatorConfig ??= MapBuilder<String, JsonObject?>();
  set operatorConfig(MapBuilder<String, JsonObject?>? operatorConfig) =>
      _$this._operatorConfig = operatorConfig;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  AgentBuilder() {
    Agent._defaults(this);
  }

  AgentBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _handle = $v.handle;
      _displayName = $v.displayName;
      _avatarFileId = $v.avatarFileId;
      _status = $v.status;
      _adapterKind = $v.adapterKind;
      _description = $v.description;
      _billingMode = $v.billingMode;
      _credentialHolderId = $v.credentialHolderId;
      _billingModeChangedAt = $v.billingModeChangedAt;
      _toolClass = $v.toolClass;
      _presence = $v.presence.toBuilder();
      _operatorConfig = $v.operatorConfig.toBuilder();
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Agent other) {
    _$v = other as _$Agent;
  }

  @override
  void update(void Function(AgentBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Agent build() => _build();

  _$Agent _build() {
    _$Agent _$result;
    try {
      _$result =
          _$v ??
          _$Agent._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Agent', 'id'),
            handle: BuiltValueNullFieldError.checkNotNull(
              handle,
              r'Agent',
              'handle',
            ),
            displayName: BuiltValueNullFieldError.checkNotNull(
              displayName,
              r'Agent',
              'displayName',
            ),
            avatarFileId: avatarFileId,
            status: BuiltValueNullFieldError.checkNotNull(
              status,
              r'Agent',
              'status',
            ),
            adapterKind: BuiltValueNullFieldError.checkNotNull(
              adapterKind,
              r'Agent',
              'adapterKind',
            ),
            description: BuiltValueNullFieldError.checkNotNull(
              description,
              r'Agent',
              'description',
            ),
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'Agent',
              'billingMode',
            ),
            credentialHolderId: credentialHolderId,
            billingModeChangedAt: billingModeChangedAt,
            toolClass: BuiltValueNullFieldError.checkNotNull(
              toolClass,
              r'Agent',
              'toolClass',
            ),
            presence: presence.build(),
            operatorConfig: operatorConfig.build(),
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Agent',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'presence';
        presence.build();
        _$failedField = 'operatorConfig';
        operatorConfig.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Agent', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
