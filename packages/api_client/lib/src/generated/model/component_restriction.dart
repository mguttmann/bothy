//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'component_restriction.g.dart';

/// `run_starter`: only the person who started the run, or an owner. `owner`: only owners.
class ComponentRestriction extends EnumClass {
  @BuiltValueEnumConst(wireName: r'any')
  static const ComponentRestriction any = _$any;
  @BuiltValueEnumConst(wireName: r'run_starter')
  static const ComponentRestriction runStarter = _$runStarter;
  @BuiltValueEnumConst(wireName: r'owner')
  static const ComponentRestriction owner = _$owner;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ComponentRestriction unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<ComponentRestriction> get serializer =>
      _$componentRestrictionSerializer;

  const ComponentRestriction._(String name) : super(name);

  static BuiltSet<ComponentRestriction> get values => _$values;
  static ComponentRestriction valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ComponentRestrictionMixin = Object
    with _$ComponentRestrictionMixin;
