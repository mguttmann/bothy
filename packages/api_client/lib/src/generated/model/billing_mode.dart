//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'billing_mode.g.dart';

/// Backend policy of an agent. `personal` runs on the subscription of exactly one person.
class BillingMode extends EnumClass {
  @BuiltValueEnumConst(wireName: r'personal')
  static const BillingMode personal = _$personal;
  @BuiltValueEnumConst(wireName: r'organization')
  static const BillingMode organization = _$organization;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const BillingMode unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<BillingMode> get serializer => _$billingModeSerializer;

  const BillingMode._(String name) : super(name);

  static BuiltSet<BillingMode> get values => _$values;
  static BillingMode valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class BillingModeMixin = Object with _$BillingModeMixin;
