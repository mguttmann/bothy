//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'access_target_kind.g.dart';

/// `host` appears only in `access.changed` and `access.expiring` (terminal right, no level).
class AccessTargetKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'space')
  static const AccessTargetKind space = _$space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const AccessTargetKind channel = _$channel;
  @BuiltValueEnumConst(wireName: r'host')
  static const AccessTargetKind host = _$host;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AccessTargetKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<AccessTargetKind> get serializer =>
      _$accessTargetKindSerializer;

  const AccessTargetKind._(String name) : super(name);

  static BuiltSet<AccessTargetKind> get values => _$values;
  static AccessTargetKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class AccessTargetKindMixin = Object with _$AccessTargetKindMixin;
