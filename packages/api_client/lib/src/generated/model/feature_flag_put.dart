//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/flag_rollout.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'feature_flag_put.g.dart';

/// FeatureFlagPut
///
/// Properties:
/// * [enabled]
/// * [rollout]
/// * [reason]
@BuiltValue()
abstract class FeatureFlagPut
    implements Built<FeatureFlagPut, FeatureFlagPutBuilder> {
  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  @BuiltValueField(wireName: r'rollout')
  FlagRollout? get rollout;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  FeatureFlagPut._();

  factory FeatureFlagPut([void updates(FeatureFlagPutBuilder b)]) =
      _$FeatureFlagPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FeatureFlagPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FeatureFlagPut> get serializer =>
      _$FeatureFlagPutSerializer();
}

class _$FeatureFlagPutSerializer
    implements PrimitiveSerializer<FeatureFlagPut> {
  @override
  final Iterable<Type> types = const [FeatureFlagPut, _$FeatureFlagPut];

  @override
  final String wireName = r'FeatureFlagPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FeatureFlagPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'enabled';
    yield serializers.serialize(
      object.enabled,
      specifiedType: const FullType(bool),
    );
    if (object.rollout != null) {
      yield r'rollout';
      yield serializers.serialize(
        object.rollout,
        specifiedType: const FullType(FlagRollout),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    FeatureFlagPut object, {
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
    required FeatureFlagPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.enabled = valueDes;
          break;
        case r'rollout':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(FlagRollout),
          ) as FlagRollout?;
          if (valueDes == null) continue;
          result.rollout.replace(valueDes);
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  FeatureFlagPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FeatureFlagPutBuilder();
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
