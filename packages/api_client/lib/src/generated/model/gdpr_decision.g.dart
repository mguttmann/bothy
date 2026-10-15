// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_decision.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GdprDecisionDecisionEnum _$gdprDecisionDecisionEnum_approve =
    const GdprDecisionDecisionEnum._('approve');
const GdprDecisionDecisionEnum _$gdprDecisionDecisionEnum_reject =
    const GdprDecisionDecisionEnum._('reject');
const GdprDecisionDecisionEnum
_$gdprDecisionDecisionEnum_unknownDefaultOpenApi =
    const GdprDecisionDecisionEnum._('unknownDefaultOpenApi');

GdprDecisionDecisionEnum _$gdprDecisionDecisionEnumValueOf(String name) {
  switch (name) {
    case 'approve':
      return _$gdprDecisionDecisionEnum_approve;
    case 'reject':
      return _$gdprDecisionDecisionEnum_reject;
    case 'unknownDefaultOpenApi':
      return _$gdprDecisionDecisionEnum_unknownDefaultOpenApi;
    default:
      return _$gdprDecisionDecisionEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<GdprDecisionDecisionEnum> _$gdprDecisionDecisionEnumValues =
    BuiltSet<GdprDecisionDecisionEnum>(const <GdprDecisionDecisionEnum>[
      _$gdprDecisionDecisionEnum_approve,
      _$gdprDecisionDecisionEnum_reject,
      _$gdprDecisionDecisionEnum_unknownDefaultOpenApi,
    ]);

Serializer<GdprDecisionDecisionEnum> _$gdprDecisionDecisionEnumSerializer =
    _$GdprDecisionDecisionEnumSerializer();

class _$GdprDecisionDecisionEnumSerializer
    implements PrimitiveSerializer<GdprDecisionDecisionEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'approve': 'approve',
    'reject': 'reject',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'approve': 'approve',
    'reject': 'reject',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GdprDecisionDecisionEnum];
  @override
  final String wireName = 'GdprDecisionDecisionEnum';

  @override
  Object serialize(
    Serializers serializers,
    GdprDecisionDecisionEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GdprDecisionDecisionEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GdprDecisionDecisionEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$GdprDecision extends GdprDecision {
  @override
  final GdprDecisionDecisionEnum decision;
  @override
  final String reason;

  factory _$GdprDecision([void Function(GdprDecisionBuilder)? updates]) =>
      (GdprDecisionBuilder()..update(updates))._build();

  _$GdprDecision._({required this.decision, required this.reason}) : super._();
  @override
  GdprDecision rebuild(void Function(GdprDecisionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GdprDecisionBuilder toBuilder() => GdprDecisionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprDecision &&
        decision == other.decision &&
        reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, decision.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GdprDecision')
          ..add('decision', decision)
          ..add('reason', reason))
        .toString();
  }
}

class GdprDecisionBuilder
    implements Builder<GdprDecision, GdprDecisionBuilder> {
  _$GdprDecision? _$v;

  GdprDecisionDecisionEnum? _decision;
  GdprDecisionDecisionEnum? get decision => _$this._decision;
  set decision(GdprDecisionDecisionEnum? decision) =>
      _$this._decision = decision;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  GdprDecisionBuilder() {
    GdprDecision._defaults(this);
  }

  GdprDecisionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _decision = $v.decision;
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprDecision other) {
    _$v = other as _$GdprDecision;
  }

  @override
  void update(void Function(GdprDecisionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprDecision build() => _build();

  _$GdprDecision _build() {
    final _$result =
        _$v ??
        _$GdprDecision._(
          decision: BuiltValueNullFieldError.checkNotNull(
            decision,
            r'GdprDecision',
            'decision',
          ),
          reason: BuiltValueNullFieldError.checkNotNull(
            reason,
            r'GdprDecision',
            'reason',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
