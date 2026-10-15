//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/user_setting.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'user_setting_list.g.dart';

/// UserSettingList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class UserSettingList
    implements Built<UserSettingList, UserSettingListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<UserSetting> get items;

  UserSettingList._();

  factory UserSettingList([void updates(UserSettingListBuilder b)]) =
      _$UserSettingList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UserSettingListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UserSettingList> get serializer =>
      _$UserSettingListSerializer();
}

class _$UserSettingListSerializer
    implements PrimitiveSerializer<UserSettingList> {
  @override
  final Iterable<Type> types = const [UserSettingList, _$UserSettingList];

  @override
  final String wireName = r'UserSettingList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UserSettingList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(UserSetting)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UserSettingList object, {
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
    required UserSettingListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(UserSetting)]),
          ) as BuiltList<UserSetting>;
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
  UserSettingList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UserSettingListBuilder();
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
