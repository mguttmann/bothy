//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/notification_preference.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_preference_list.g.dart';

/// NotificationPreferenceList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class NotificationPreferenceList
    implements
        Built<NotificationPreferenceList, NotificationPreferenceListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<NotificationPreference> get items;

  NotificationPreferenceList._();

  factory NotificationPreferenceList([
    void updates(NotificationPreferenceListBuilder b),
  ]) = _$NotificationPreferenceList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(NotificationPreferenceListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<NotificationPreferenceList> get serializer =>
      _$NotificationPreferenceListSerializer();
}

class _$NotificationPreferenceListSerializer
    implements PrimitiveSerializer<NotificationPreferenceList> {
  @override
  final Iterable<Type> types = const [
    NotificationPreferenceList,
    _$NotificationPreferenceList,
  ];

  @override
  final String wireName = r'NotificationPreferenceList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    NotificationPreferenceList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [
        FullType(NotificationPreference),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    NotificationPreferenceList object, {
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
    required NotificationPreferenceListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(NotificationPreference),
            ]),
          ) as BuiltList<NotificationPreference>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  NotificationPreferenceList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = NotificationPreferenceListBuilder();
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
