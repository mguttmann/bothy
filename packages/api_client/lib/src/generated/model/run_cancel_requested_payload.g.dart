// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_cancel_requested_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_userCancelled =
    const RunCancelRequestedPayloadReasonEnum._('userCancelled');
const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_accessRevoked =
    const RunCancelRequestedPayloadReasonEnum._('accessRevoked');
const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_agentDisabled =
    const RunCancelRequestedPayloadReasonEnum._('agentDisabled');
const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_userDisabled =
    const RunCancelRequestedPayloadReasonEnum._('userDisabled');
const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_channelReadOnly =
    const RunCancelRequestedPayloadReasonEnum._('channelReadOnly');
const RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnum_unknownDefaultOpenApi =
    const RunCancelRequestedPayloadReasonEnum._('unknownDefaultOpenApi');

RunCancelRequestedPayloadReasonEnum
_$runCancelRequestedPayloadReasonEnumValueOf(String name) {
  switch (name) {
    case 'userCancelled':
      return _$runCancelRequestedPayloadReasonEnum_userCancelled;
    case 'accessRevoked':
      return _$runCancelRequestedPayloadReasonEnum_accessRevoked;
    case 'agentDisabled':
      return _$runCancelRequestedPayloadReasonEnum_agentDisabled;
    case 'userDisabled':
      return _$runCancelRequestedPayloadReasonEnum_userDisabled;
    case 'channelReadOnly':
      return _$runCancelRequestedPayloadReasonEnum_channelReadOnly;
    case 'unknownDefaultOpenApi':
      return _$runCancelRequestedPayloadReasonEnum_unknownDefaultOpenApi;
    default:
      return _$runCancelRequestedPayloadReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunCancelRequestedPayloadReasonEnum>
_$runCancelRequestedPayloadReasonEnumValues =
    BuiltSet<RunCancelRequestedPayloadReasonEnum>(
      const <RunCancelRequestedPayloadReasonEnum>[
        _$runCancelRequestedPayloadReasonEnum_userCancelled,
        _$runCancelRequestedPayloadReasonEnum_accessRevoked,
        _$runCancelRequestedPayloadReasonEnum_agentDisabled,
        _$runCancelRequestedPayloadReasonEnum_userDisabled,
        _$runCancelRequestedPayloadReasonEnum_channelReadOnly,
        _$runCancelRequestedPayloadReasonEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<RunCancelRequestedPayloadReasonEnum>
_$runCancelRequestedPayloadReasonEnumSerializer =
    _$RunCancelRequestedPayloadReasonEnumSerializer();

class _$RunCancelRequestedPayloadReasonEnumSerializer
    implements PrimitiveSerializer<RunCancelRequestedPayloadReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'userCancelled': 'user_cancelled',
    'accessRevoked': 'access_revoked',
    'agentDisabled': 'agent_disabled',
    'userDisabled': 'user_disabled',
    'channelReadOnly': 'channel_read_only',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'user_cancelled': 'userCancelled',
    'access_revoked': 'accessRevoked',
    'agent_disabled': 'agentDisabled',
    'user_disabled': 'userDisabled',
    'channel_read_only': 'channelReadOnly',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    RunCancelRequestedPayloadReasonEnum,
  ];
  @override
  final String wireName = 'RunCancelRequestedPayloadReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunCancelRequestedPayloadReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunCancelRequestedPayloadReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunCancelRequestedPayloadReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunCancelRequestedPayload extends RunCancelRequestedPayload {
  @override
  final String runId;
  @override
  final String threadId;
  @override
  final RunCancelRequestedPayloadReasonEnum reason;
  @override
  final String? requestedBy;

  factory _$RunCancelRequestedPayload([
    void Function(RunCancelRequestedPayloadBuilder)? updates,
  ]) => (RunCancelRequestedPayloadBuilder()..update(updates))._build();

  _$RunCancelRequestedPayload._({
    required this.runId,
    required this.threadId,
    required this.reason,
    this.requestedBy,
  }) : super._();
  @override
  RunCancelRequestedPayload rebuild(
    void Function(RunCancelRequestedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunCancelRequestedPayloadBuilder toBuilder() =>
      RunCancelRequestedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunCancelRequestedPayload &&
        runId == other.runId &&
        threadId == other.threadId &&
        reason == other.reason &&
        requestedBy == other.requestedBy;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, requestedBy.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunCancelRequestedPayload')
          ..add('runId', runId)
          ..add('threadId', threadId)
          ..add('reason', reason)
          ..add('requestedBy', requestedBy))
        .toString();
  }
}

class RunCancelRequestedPayloadBuilder
    implements
        Builder<RunCancelRequestedPayload, RunCancelRequestedPayloadBuilder> {
  _$RunCancelRequestedPayload? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  RunCancelRequestedPayloadReasonEnum? _reason;
  RunCancelRequestedPayloadReasonEnum? get reason => _$this._reason;
  set reason(RunCancelRequestedPayloadReasonEnum? reason) =>
      _$this._reason = reason;

  String? _requestedBy;
  String? get requestedBy => _$this._requestedBy;
  set requestedBy(String? requestedBy) => _$this._requestedBy = requestedBy;

  RunCancelRequestedPayloadBuilder() {
    RunCancelRequestedPayload._defaults(this);
  }

  RunCancelRequestedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _threadId = $v.threadId;
      _reason = $v.reason;
      _requestedBy = $v.requestedBy;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunCancelRequestedPayload other) {
    _$v = other as _$RunCancelRequestedPayload;
  }

  @override
  void update(void Function(RunCancelRequestedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunCancelRequestedPayload build() => _build();

  _$RunCancelRequestedPayload _build() {
    final _$result =
        _$v ??
        _$RunCancelRequestedPayload._(
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'RunCancelRequestedPayload',
            'runId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'RunCancelRequestedPayload',
            'threadId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'RunCancelRequestedPayload',
            'reason',
          ),
          requestedBy: requestedBy,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
