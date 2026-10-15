//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'target_kind.g.dart';

/// `host` is a connected host for the terminal; it has no level and only the capability `terminal` (24, 8.1).
class TargetKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'instance')
  static const TargetKind instance = _$instance;
  @BuiltValueEnumConst(wireName: r'space')
  static const TargetKind space = _$space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const TargetKind channel = _$channel;
  @BuiltValueEnumConst(wireName: r'host')
  static const TargetKind host = _$host;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TargetKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<TargetKind> get serializer => _$targetKindSerializer;

  const TargetKind._(String name) : super(name);

  static BuiltSet<TargetKind> get values => _$values;
  static TargetKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TargetKindMixin = Object with _$TargetKindMixin;
