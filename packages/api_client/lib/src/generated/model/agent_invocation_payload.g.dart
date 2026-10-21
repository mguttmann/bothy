// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_invocation_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentInvocationPayloadTriggerKindEnum
_$agentInvocationPayloadTriggerKindEnum_message =
    const AgentInvocationPayloadTriggerKindEnum._('message');
const AgentInvocationPayloadTriggerKindEnum
_$agentInvocationPayloadTriggerKindEnum_interaction =
    const AgentInvocationPayloadTriggerKindEnum._('interaction');
const AgentInvocationPayloadTriggerKindEnum
_$agentInvocationPayloadTriggerKindEnum_command =
    const AgentInvocationPayloadTriggerKindEnum._('command');
const AgentInvocationPayloadTriggerKindEnum
_$agentInvocationPayloadTriggerKindEnum_unknownDefaultOpenApi =
    const AgentInvocationPayloadTriggerKindEnum._('unknownDefaultOpenApi');

AgentInvocationPayloadTriggerKindEnum
_$agentInvocationPayloadTriggerKindEnumValueOf(String name) {
  switch (name) {
    case 'message':
      return _$agentInvocationPayloadTriggerKindEnum_message;
    case 'interaction':
      return _$agentInvocationPayloadTriggerKindEnum_interaction;
    case 'command':
      return _$agentInvocationPayloadTriggerKindEnum_command;
    case 'unknownDefaultOpenApi':
      return _$agentInvocationPayloadTriggerKindEnum_unknownDefaultOpenApi;
    default:
      return _$agentInvocationPayloadTriggerKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInvocationPayloadTriggerKindEnum>
_$agentInvocationPayloadTriggerKindEnumValues =
    BuiltSet<AgentInvocationPayloadTriggerKindEnum>(
      const <AgentInvocationPayloadTriggerKindEnum>[
        _$agentInvocationPayloadTriggerKindEnum_message,
        _$agentInvocationPayloadTriggerKindEnum_interaction,
        _$agentInvocationPayloadTriggerKindEnum_command,
        _$agentInvocationPayloadTriggerKindEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentInvocationPayloadTriggerKindEnum>
_$agentInvocationPayloadTriggerKindEnumSerializer =
    _$AgentInvocationPayloadTriggerKindEnumSerializer();

class _$AgentInvocationPayloadTriggerKindEnumSerializer
    implements PrimitiveSerializer<AgentInvocationPayloadTriggerKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'message': 'message',
    'interaction': 'interaction',
    'command': 'command',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'message': 'message',
    'interaction': 'interaction',
    'command': 'command',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    AgentInvocationPayloadTriggerKindEnum,
  ];
  @override
  final String wireName = 'AgentInvocationPayloadTriggerKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInvocationPayloadTriggerKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInvocationPayloadTriggerKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInvocationPayloadTriggerKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInvocationPayload extends AgentInvocationPayload {
  @override
  final String runId;
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String? spaceId;
  @override
  final AgentInvocationPayloadTriggerKindEnum triggerKind;
  @override
  final String? messageId;
  @override
  final String? interactionId;
  @override
  final String? commandInvocationId;
  @override
  final String? commandId;
  @override
  final String? commandName;
  @override
  final BuiltList<CommandOptionValue>? options;
  @override
  final PrincipalRef triggeredBy;
  @override
  final BillingMode billingMode;
  @override
  final AgentSession? agentSession;
  @override
  final int? settingsVersion;

  factory _$AgentInvocationPayload([
    void Function(AgentInvocationPayloadBuilder)? updates,
  ]) => (AgentInvocationPayloadBuilder()..update(updates))._build();

  _$AgentInvocationPayload._({
    required this.runId,
    required this.threadId,
    required this.channelId,
    this.spaceId,
    required this.triggerKind,
    this.messageId,
    this.interactionId,
    this.commandInvocationId,
    this.commandId,
    this.commandName,
    this.options,
    required this.triggeredBy,
    required this.billingMode,
    this.agentSession,
    this.settingsVersion,
  }) : super._();
  @override
  AgentInvocationPayload rebuild(
    void Function(AgentInvocationPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInvocationPayloadBuilder toBuilder() =>
      AgentInvocationPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInvocationPayload &&
        runId == other.runId &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        spaceId == other.spaceId &&
        triggerKind == other.triggerKind &&
        messageId == other.messageId &&
        interactionId == other.interactionId &&
        commandInvocationId == other.commandInvocationId &&
        commandId == other.commandId &&
        commandName == other.commandName &&
        options == other.options &&
        triggeredBy == other.triggeredBy &&
        billingMode == other.billingMode &&
        agentSession == other.agentSession &&
        settingsVersion == other.settingsVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, triggerKind.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, interactionId.hashCode);
    _$hash = $jc(_$hash, commandInvocationId.hashCode);
    _$hash = $jc(_$hash, commandId.hashCode);
    _$hash = $jc(_$hash, commandName.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jc(_$hash, triggeredBy.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, agentSession.hashCode);
    _$hash = $jc(_$hash, settingsVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInvocationPayload')
          ..add('runId', runId)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('spaceId', spaceId)
          ..add('triggerKind', triggerKind)
          ..add('messageId', messageId)
          ..add('interactionId', interactionId)
          ..add('commandInvocationId', commandInvocationId)
          ..add('commandId', commandId)
          ..add('commandName', commandName)
          ..add('options', options)
          ..add('triggeredBy', triggeredBy)
          ..add('billingMode', billingMode)
          ..add('agentSession', agentSession)
          ..add('settingsVersion', settingsVersion))
        .toString();
  }
}

class AgentInvocationPayloadBuilder
    implements Builder<AgentInvocationPayload, AgentInvocationPayloadBuilder> {
  _$AgentInvocationPayload? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  AgentInvocationPayloadTriggerKindEnum? _triggerKind;
  AgentInvocationPayloadTriggerKindEnum? get triggerKind => _$this._triggerKind;
  set triggerKind(AgentInvocationPayloadTriggerKindEnum? triggerKind) =>
      _$this._triggerKind = triggerKind;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _interactionId;
  String? get interactionId => _$this._interactionId;
  set interactionId(String? interactionId) =>
      _$this._interactionId = interactionId;

  String? _commandInvocationId;
  String? get commandInvocationId => _$this._commandInvocationId;
  set commandInvocationId(String? commandInvocationId) =>
      _$this._commandInvocationId = commandInvocationId;

  String? _commandId;
  String? get commandId => _$this._commandId;
  set commandId(String? commandId) => _$this._commandId = commandId;

  String? _commandName;
  String? get commandName => _$this._commandName;
  set commandName(String? commandName) => _$this._commandName = commandName;

  ListBuilder<CommandOptionValue>? _options;
  ListBuilder<CommandOptionValue> get options =>
      _$this._options ??= ListBuilder<CommandOptionValue>();
  set options(ListBuilder<CommandOptionValue>? options) =>
      _$this._options = options;

  PrincipalRefBuilder? _triggeredBy;
  PrincipalRefBuilder get triggeredBy =>
      _$this._triggeredBy ??= PrincipalRefBuilder();
  set triggeredBy(PrincipalRefBuilder? triggeredBy) =>
      _$this._triggeredBy = triggeredBy;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  AgentSessionBuilder? _agentSession;
  AgentSessionBuilder get agentSession =>
      _$this._agentSession ??= AgentSessionBuilder();
  set agentSession(AgentSessionBuilder? agentSession) =>
      _$this._agentSession = agentSession;

  int? _settingsVersion;
  int? get settingsVersion => _$this._settingsVersion;
  set settingsVersion(int? settingsVersion) =>
      _$this._settingsVersion = settingsVersion;

  AgentInvocationPayloadBuilder() {
    AgentInvocationPayload._defaults(this);
  }

  AgentInvocationPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _spaceId = $v.spaceId;
      _triggerKind = $v.triggerKind;
      _messageId = $v.messageId;
      _interactionId = $v.interactionId;
      _commandInvocationId = $v.commandInvocationId;
      _commandId = $v.commandId;
      _commandName = $v.commandName;
      _options = $v.options?.toBuilder();
      _triggeredBy = $v.triggeredBy.toBuilder();
      _billingMode = $v.billingMode;
      _agentSession = $v.agentSession?.toBuilder();
      _settingsVersion = $v.settingsVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInvocationPayload other) {
    _$v = other as _$AgentInvocationPayload;
  }

  @override
  void update(void Function(AgentInvocationPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInvocationPayload build() => _build();

  _$AgentInvocationPayload _build() {
    _$AgentInvocationPayload _$result;
    try {
      _$result =
          _$v ??
          _$AgentInvocationPayload._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'AgentInvocationPayload',
              'runId',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'AgentInvocationPayload',
              'threadId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'AgentInvocationPayload',
              'channelId',
            ),
            spaceId: spaceId,
            triggerKind: BuiltValueNullFieldError.checkNotNull(
              triggerKind,
              r'AgentInvocationPayload',
              'triggerKind',
            ),
            messageId: messageId,
            interactionId: interactionId,
            commandInvocationId: commandInvocationId,
            commandId: commandId,
            commandName: commandName,
            options: _options?.build(),
            triggeredBy: triggeredBy.build(),
            billingMode: BuiltValueNullFieldError.checkNotNull(
              billingMode,
              r'AgentInvocationPayload',
              'billingMode',
            ),
            agentSession: _agentSession?.build(),
            settingsVersion: settingsVersion,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        _options?.build();
        _$failedField = 'triggeredBy';
        triggeredBy.build();

        _$failedField = 'agentSession';
        _agentSession?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInvocationPayload',
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
