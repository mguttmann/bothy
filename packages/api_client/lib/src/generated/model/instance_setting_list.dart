//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/instance_setting.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'instance_setting_list.g.dart';

/// InstanceSettingList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class InstanceSettingList
    implements Built<InstanceSettingList, InstanceSettingListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<InstanceSetting> get items;

  InstanceSettingList._();

  factory InstanceSettingList([void updates(InstanceSettingListBuilder b)]) =
      _$InstanceSettingList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InstanceSettingListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InstanceSettingList> get serializer =>
      _$InstanceSettingListSerializer();
}

class _$InstanceSettingListSerializer
    implements PrimitiveSerializer<InstanceSettingList> {
  @override
  final Iterable<Type> types = const [
    InstanceSettingList,
    _$InstanceSettingList,
  ];

  @override
  final String wireName = r'InstanceSettingList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InstanceSettingList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(InstanceSetting)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    InstanceSettingList object, {
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
    required InstanceSettingListBuilder result,
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
              FullType(InstanceSetting),
            ]),
          ) as BuiltList<InstanceSetting>;
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
  InstanceSettingList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InstanceSettingListBuilder();
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
