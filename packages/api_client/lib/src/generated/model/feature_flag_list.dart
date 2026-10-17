//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/feature_flag.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'feature_flag_list.g.dart';

/// FeatureFlagList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class FeatureFlagList
    implements Built<FeatureFlagList, FeatureFlagListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<FeatureFlag> get items;

  FeatureFlagList._();

  factory FeatureFlagList([void updates(FeatureFlagListBuilder b)]) =
      _$FeatureFlagList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FeatureFlagListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FeatureFlagList> get serializer =>
      _$FeatureFlagListSerializer();
}

class _$FeatureFlagListSerializer
    implements PrimitiveSerializer<FeatureFlagList> {
  @override
  final Iterable<Type> types = const [FeatureFlagList, _$FeatureFlagList];

  @override
  final String wireName = r'FeatureFlagList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FeatureFlagList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(FeatureFlag)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    FeatureFlagList object, {
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
    required FeatureFlagListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(FeatureFlag)]),
          ) as BuiltList<FeatureFlag>;
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
  FeatureFlagList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FeatureFlagListBuilder();
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
