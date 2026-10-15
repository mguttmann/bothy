//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_level.g.dart';

class NotificationLevel extends EnumClass {
  @BuiltValueEnumConst(wireName: r'all')
  static const NotificationLevel all = _$all;
  @BuiltValueEnumConst(wireName: r'mentions')
  static const NotificationLevel mentions = _$mentions;
  @BuiltValueEnumConst(wireName: r'mute')
  static const NotificationLevel mute = _$mute;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const NotificationLevel unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<NotificationLevel> get serializer =>
      _$notificationLevelSerializer;

  const NotificationLevel._(String name) : super(name);

  static BuiltSet<NotificationLevel> get values => _$values;
  static NotificationLevel valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class NotificationLevelMixin = Object with _$NotificationLevelMixin;
