//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device_platform.g.dart';

class DevicePlatform extends EnumClass {
  @BuiltValueEnumConst(wireName: r'ios')
  static const DevicePlatform ios = _$ios;
  @BuiltValueEnumConst(wireName: r'android')
  static const DevicePlatform android = _$android;
  @BuiltValueEnumConst(wireName: r'macos')
  static const DevicePlatform macos = _$macos;
  @BuiltValueEnumConst(wireName: r'windows')
  static const DevicePlatform windows = _$windows;
  @BuiltValueEnumConst(wireName: r'web')
  static const DevicePlatform web = _$web;
  @BuiltValueEnumConst(wireName: r'cli')
  static const DevicePlatform cli = _$cli;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DevicePlatform unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<DevicePlatform> get serializer =>
      _$devicePlatformSerializer;

  const DevicePlatform._(String name) : super(name);

  static BuiltSet<DevicePlatform> get values => _$values;
  static DevicePlatform valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class DevicePlatformMixin = Object with _$DevicePlatformMixin;
