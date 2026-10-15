//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'flag_rollout.g.dart';

/// Principals and groups for which the flag has the opposite value of `enabled`.
///
/// Properties:
/// * [userIds]
/// * [groupIds]
@BuiltValue()
abstract class FlagRollout implements Built<FlagRollout, FlagRolloutBuilder> {
  @BuiltValueField(wireName: r'user_ids')
  BuiltList<String>? get userIds;

  @BuiltValueField(wireName: r'group_ids')
  BuiltList<String>? get groupIds;

  FlagRollout._();

  factory FlagRollout([void updates(FlagRolloutBuilder b)]) = _$FlagRollout;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FlagRolloutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FlagRollout> get serializer => _$FlagRolloutSerializer();
}

class _$FlagRolloutSerializer implements PrimitiveSerializer<FlagRollout> {
  @override
  final Iterable<Type> types = const [FlagRollout, _$FlagRollout];

  @override
  final String wireName = r'FlagRollout';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FlagRollout object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.userIds != null) {
      yield r'user_ids';
      yield serializers.serialize(
        object.userIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.groupIds != null) {
      yield r'group_ids';
      yield serializers.serialize(
        object.groupIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    FlagRollout object, {
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
    required FlagRolloutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'user_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.userIds.replace(valueDes);
          break;
        case r'group_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.groupIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  FlagRollout deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FlagRolloutBuilder();
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
