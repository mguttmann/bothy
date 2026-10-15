//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_kind.g.dart';

class NotificationKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'message')
  static const NotificationKind message = _$message;
  @BuiltValueEnumConst(wireName: r'mention')
  static const NotificationKind mention = _$mention;
  @BuiltValueEnumConst(wireName: r'dm')
  static const NotificationKind dm = _$dm;
  @BuiltValueEnumConst(wireName: r'run_done')
  static const NotificationKind runDone = _$runDone;
  @BuiltValueEnumConst(wireName: r'run_failed')
  static const NotificationKind runFailed = _$runFailed;
  @BuiltValueEnumConst(wireName: r'input_needed')
  static const NotificationKind inputNeeded = _$inputNeeded;
  @BuiltValueEnumConst(wireName: r'security')
  static const NotificationKind security = _$security;
  @BuiltValueEnumConst(wireName: r'admin')
  static const NotificationKind admin = _$admin;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const NotificationKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<NotificationKind> get serializer =>
      _$notificationKindSerializer;

  const NotificationKind._(String name) : super(name);

  static BuiltSet<NotificationKind> get values => _$values;
  static NotificationKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class NotificationKindMixin = Object with _$NotificationKindMixin;
