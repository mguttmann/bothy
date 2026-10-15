//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'audit_decision.g.dart';

class AuditDecision extends EnumClass {
  @BuiltValueEnumConst(wireName: r'allow')
  static const AuditDecision allow = _$allow;
  @BuiltValueEnumConst(wireName: r'deny')
  static const AuditDecision deny = _$deny;
  @BuiltValueEnumConst(wireName: r'error')
  static const AuditDecision error = _$error;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AuditDecision unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<AuditDecision> get serializer => _$auditDecisionSerializer;

  const AuditDecision._(String name) : super(name);

  static BuiltSet<AuditDecision> get values => _$values;
  static AuditDecision valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class AuditDecisionMixin = Object with _$AuditDecisionMixin;
