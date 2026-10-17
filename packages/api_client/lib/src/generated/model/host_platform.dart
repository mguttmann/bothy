//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_platform.g.dart';

class HostPlatform extends EnumClass {
  @BuiltValueEnumConst(wireName: r'macos')
  static const HostPlatform macos = _$macos;
  @BuiltValueEnumConst(wireName: r'linux')
  static const HostPlatform linux = _$linux;
  @BuiltValueEnumConst(wireName: r'windows')
  static const HostPlatform windows = _$windows;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostPlatform unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<HostPlatform> get serializer => _$hostPlatformSerializer;

  const HostPlatform._(String name) : super(name);

  static BuiltSet<HostPlatform> get values => _$values;
  static HostPlatform valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class HostPlatformMixin = Object with _$HostPlatformMixin;
