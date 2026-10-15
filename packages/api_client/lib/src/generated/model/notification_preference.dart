//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/notification_level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_preference.g.dart';

/// NotificationPreference
///
/// Properties:
/// * [targetKind]
/// * [targetId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [level]
/// * [mutedUntil] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
@BuiltValue()
abstract class NotificationPreference
    implements Built<NotificationPreference, NotificationPreferenceBuilder> {
  @BuiltValueField(wireName: r'target_kind')
  NotificationPreferenceTargetKindEnum get targetKind;
  // enum targetKindEnum {  space,  channel,  thread,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'target_id')
  String get targetId;

  @BuiltValueField(wireName: r'level')
  NotificationLevel get level;
  // enum levelEnum {  all,  mentions,  mute,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'muted_until')
  DateTime? get mutedUntil;

  @BuiltValueField(wireName: r'version')
  int get version;

  NotificationPreference._();

  factory NotificationPreference([
    void updates(NotificationPreferenceBuilder b),
  ]) = _$NotificationPreference;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(NotificationPreferenceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<NotificationPreference> get serializer =>
      _$NotificationPreferenceSerializer();
}

class _$NotificationPreferenceSerializer
    implements PrimitiveSerializer<NotificationPreference> {
  @override
  final Iterable<Type> types = const [
    NotificationPreference,
    _$NotificationPreference,
  ];

  @override
  final String wireName = r'NotificationPreference';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    NotificationPreference object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'target_kind';
    yield serializers.serialize(
      object.targetKind,
      specifiedType: const FullType(NotificationPreferenceTargetKindEnum),
    );
    yield r'target_id';
    yield serializers.serialize(
      object.targetId,
      specifiedType: const FullType(String),
    );
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(NotificationLevel),
    );
    if (object.mutedUntil != null) {
      yield r'muted_until';
      yield serializers.serialize(
        object.mutedUntil,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    NotificationPreference object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required NotificationPreferenceBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'target_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(NotificationPreferenceTargetKindEnum),
          ) as NotificationPreferenceTargetKindEnum;
          result.targetKind = valueDes;
          break;
        case r'target_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.targetId = valueDes;
          break;
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(NotificationLevel),
          ) as NotificationLevel;
          result.level = valueDes;
          break;
        case r'muted_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.mutedUntil = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  NotificationPreference deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = NotificationPreferenceBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

class NotificationPreferenceTargetKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'space')
  static const NotificationPreferenceTargetKindEnum space =
      _$notificationPreferenceTargetKindEnum_space;
  @BuiltValueEnumConst(wireName: r'channel')
  static const NotificationPreferenceTargetKindEnum channel =
      _$notificationPreferenceTargetKindEnum_channel;
  @BuiltValueEnumConst(wireName: r'thread')
  static const NotificationPreferenceTargetKindEnum thread =
      _$notificationPreferenceTargetKindEnum_thread;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const NotificationPreferenceTargetKindEnum unknownDefaultOpenApi =
      _$notificationPreferenceTargetKindEnum_unknownDefaultOpenApi;

  static Serializer<NotificationPreferenceTargetKindEnum> get serializer =>
      _$notificationPreferenceTargetKindEnumSerializer;

  const NotificationPreferenceTargetKindEnum._(String name) : super(name);

  static BuiltSet<NotificationPreferenceTargetKindEnum> get values =>
      _$notificationPreferenceTargetKindEnumValues;
  static NotificationPreferenceTargetKindEnum valueOf(String name) =>
      _$notificationPreferenceTargetKindEnumValueOf(name);
}
