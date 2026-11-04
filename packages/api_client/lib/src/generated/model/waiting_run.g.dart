// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'waiting_run.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const WaitingRunReasonEnum _$waitingRunReasonEnum_slotsFull =
    const WaitingRunReasonEnum._('slotsFull');
const WaitingRunReasonEnum _$waitingRunReasonEnum_threadBusy =
    const WaitingRunReasonEnum._('threadBusy');
const WaitingRunReasonEnum _$waitingRunReasonEnum_unknownDefaultOpenApi =
    const WaitingRunReasonEnum._('unknownDefaultOpenApi');

WaitingRunReasonEnum _$waitingRunReasonEnumValueOf(String name) {
  switch (name) {
    case 'slotsFull':
      return _$waitingRunReasonEnum_slotsFull;
    case 'threadBusy':
      return _$waitingRunReasonEnum_threadBusy;
    case 'unknownDefaultOpenApi':
      return _$waitingRunReasonEnum_unknownDefaultOpenApi;
    default:
      return _$waitingRunReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<WaitingRunReasonEnum> _$waitingRunReasonEnumValues =
    BuiltSet<WaitingRunReasonEnum>(const <WaitingRunReasonEnum>[
      _$waitingRunReasonEnum_slotsFull,
      _$waitingRunReasonEnum_threadBusy,
      _$waitingRunReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<WaitingRunReasonEnum> _$waitingRunReasonEnumSerializer =
    _$WaitingRunReasonEnumSerializer();

class _$WaitingRunReasonEnumSerializer
    implements PrimitiveSerializer<WaitingRunReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'slotsFull': 'slots_full',
    'threadBusy': 'thread_busy',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'slots_full': 'slotsFull',
    'thread_busy': 'threadBusy',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[WaitingRunReasonEnum];
  @override
  final String wireName = 'WaitingRunReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    WaitingRunReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  WaitingRunReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => WaitingRunReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$WaitingRun extends WaitingRun {
  @override
  final String runId;
  @override
  final WaitingRunReasonEnum reason;

  factory _$WaitingRun([void Function(WaitingRunBuilder)? updates]) =>
      (WaitingRunBuilder()..update(updates))._build();

  _$WaitingRun._({required this.runId, required this.reason}) : super._();
  @override
  WaitingRun rebuild(void Function(WaitingRunBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  WaitingRunBuilder toBuilder() => WaitingRunBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WaitingRun &&
        runId == other.runId &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'WaitingRun')
          ..add('runId', runId)
          ..add('reason', reason))
        .toString();
  }
}

class WaitingRunBuilder implements Builder<WaitingRun, WaitingRunBuilder> {
  _$WaitingRun? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  WaitingRunReasonEnum? _reason;
  WaitingRunReasonEnum? get reason => _$this._reason;
  set reason(WaitingRunReasonEnum? reason) => _$this._reason = reason;

  WaitingRunBuilder() {
    WaitingRun._defaults(this);
  }

  WaitingRunBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WaitingRun other) {
    _$v = other as _$WaitingRun;
  }

  @override
  void update(void Function(WaitingRunBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WaitingRun build() => _build();

  _$WaitingRun _build() {
    final _$result =
        _$v ??
        _$WaitingRun._(
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'WaitingRun',
            'runId',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'WaitingRun',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
