// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_recover.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeRecoverReasonEnum _$runWakeRecoverReasonEnum_recovery =
    const RunWakeRecoverReasonEnum._('recovery');
const RunWakeRecoverReasonEnum _$runWakeRecoverReasonEnum_usageLimit =
    const RunWakeRecoverReasonEnum._('usageLimit');
const RunWakeRecoverReasonEnum
_$runWakeRecoverReasonEnum_unknownDefaultOpenApi =
    const RunWakeRecoverReasonEnum._('unknownDefaultOpenApi');

RunWakeRecoverReasonEnum _$runWakeRecoverReasonEnumValueOf(String name) {
  switch (name) {
    case 'recovery':
      return _$runWakeRecoverReasonEnum_recovery;
    case 'usageLimit':
      return _$runWakeRecoverReasonEnum_usageLimit;
    case 'unknownDefaultOpenApi':
      return _$runWakeRecoverReasonEnum_unknownDefaultOpenApi;
    default:
      return _$runWakeRecoverReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeRecoverReasonEnum> _$runWakeRecoverReasonEnumValues =
    BuiltSet<RunWakeRecoverReasonEnum>(const <RunWakeRecoverReasonEnum>[
      _$runWakeRecoverReasonEnum_recovery,
      _$runWakeRecoverReasonEnum_usageLimit,
      _$runWakeRecoverReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunWakeRecoverReasonEnum> _$runWakeRecoverReasonEnumSerializer =
    _$RunWakeRecoverReasonEnumSerializer();

class _$RunWakeRecoverReasonEnumSerializer
    implements PrimitiveSerializer<RunWakeRecoverReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'recovery': 'recovery',
    'usageLimit': 'usage_limit',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'recovery': 'recovery',
    'usage_limit': 'usageLimit',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeRecoverReasonEnum];
  @override
  final String wireName = 'RunWakeRecoverReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeRecoverReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeRecoverReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeRecoverReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunWakeRecover extends RunWakeRecover {
  @override
  final RunWakeRecoverReasonEnum reason;
  @override
  final String? label;
  @override
  final DateTime? notBefore;

  factory _$RunWakeRecover([void Function(RunWakeRecoverBuilder)? updates]) =>
      (RunWakeRecoverBuilder()..update(updates))._build();

  _$RunWakeRecover._({required this.reason, this.label, this.notBefore})
    : super._();
  @override
  RunWakeRecover rebuild(void Function(RunWakeRecoverBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeRecoverBuilder toBuilder() => RunWakeRecoverBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeRecover &&
        reason == other.reason &&
        label == other.label &&
        notBefore == other.notBefore;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, label.hashCode);
    _$hash = $jc(_$hash, notBefore.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeRecover')
          ..add('reason', reason)
          ..add('label', label)
          ..add('notBefore', notBefore))
        .toString();
  }
}

class RunWakeRecoverBuilder
    implements Builder<RunWakeRecover, RunWakeRecoverBuilder> {
  _$RunWakeRecover? _$v;

  RunWakeRecoverReasonEnum? _reason;
  RunWakeRecoverReasonEnum? get reason => _$this._reason;
  set reason(RunWakeRecoverReasonEnum? reason) => _$this._reason = reason;

  String? _label;
  String? get label => _$this._label;
  set label(String? label) => _$this._label = label;

  DateTime? _notBefore;
  DateTime? get notBefore => _$this._notBefore;
  set notBefore(DateTime? notBefore) => _$this._notBefore = notBefore;

  RunWakeRecoverBuilder() {
    RunWakeRecover._defaults(this);
  }

  RunWakeRecoverBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reason = $v.reason;
      _label = $v.label;
      _notBefore = $v.notBefore;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeRecover other) {
    _$v = other as _$RunWakeRecover;
  }

  @override
  void update(void Function(RunWakeRecoverBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeRecover build() => _build();

  _$RunWakeRecover _build() {
    final _$result =
        _$v ??
        _$RunWakeRecover._(
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'RunWakeRecover',
            'reason',
          ),
          label: label,
          notBefore: notBefore,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
