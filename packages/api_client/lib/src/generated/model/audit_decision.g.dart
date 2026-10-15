// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'audit_decision.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AuditDecision _$allow = const AuditDecision._('allow');
const AuditDecision _$deny = const AuditDecision._('deny');
const AuditDecision _$error = const AuditDecision._('error');
const AuditDecision _$unknownDefaultOpenApi = const AuditDecision._(
  'unknownDefaultOpenApi',
);

AuditDecision _$valueOf(String name) {
  switch (name) {
    case 'allow':
      return _$allow;
    case 'deny':
      return _$deny;
    case 'error':
      return _$error;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<AuditDecision> _$values = BuiltSet<AuditDecision>(
  const <AuditDecision>[_$allow, _$deny, _$error, _$unknownDefaultOpenApi],
);

class _$AuditDecisionMeta {
  const _$AuditDecisionMeta();
  AuditDecision get allow => _$allow;
  AuditDecision get deny => _$deny;
  AuditDecision get error => _$error;
  AuditDecision get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  AuditDecision valueOf(String name) => _$valueOf(name);
  BuiltSet<AuditDecision> get values => _$values;
}

mixin _$AuditDecisionMixin {
  // ignore: non_constant_identifier_names
  _$AuditDecisionMeta get AuditDecision => const _$AuditDecisionMeta();
}

Serializer<AuditDecision> _$auditDecisionSerializer =
    _$AuditDecisionSerializer();

class _$AuditDecisionSerializer implements PrimitiveSerializer<AuditDecision> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'allow': 'allow',
    'deny': 'deny',
    'error': 'error',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'allow': 'allow',
    'deny': 'deny',
    'error': 'error',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AuditDecision];
  @override
  final String wireName = 'AuditDecision';

  @override
  Object serialize(
    Serializers serializers,
    AuditDecision object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AuditDecision deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AuditDecision.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
