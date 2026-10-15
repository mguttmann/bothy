//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/interaction_response.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'interaction_respond.g.dart';

/// InteractionRespond
///
/// Properties:
/// * [state]
/// * [response]
@BuiltValue()
abstract class InteractionRespond
    implements Built<InteractionRespond, InteractionRespondBuilder> {
  @BuiltValueField(wireName: r'state')
  InteractionRespondStateEnum get state;
  // enum stateEnum {  acknowledged,  failed,  rejected,  };

  @BuiltValueField(wireName: r'response')
  InteractionResponse? get response;

  InteractionRespond._();

  factory InteractionRespond([void updates(InteractionRespondBuilder b)]) =
      _$InteractionRespond;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InteractionRespondBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InteractionRespond> get serializer =>
      _$InteractionRespondSerializer();
}

class _$InteractionRespondSerializer
    implements PrimitiveSerializer<InteractionRespond> {
  @override
  final Iterable<Type> types = const [InteractionRespond, _$InteractionRespond];

  @override
  final String wireName = r'InteractionRespond';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InteractionRespond object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(InteractionRespondStateEnum),
    );
    if (object.response != null) {
      yield r'response';
      yield serializers.serialize(
        object.response,
        specifiedType: const FullType(InteractionResponse),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    InteractionRespond object, {
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
    required InteractionRespondBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(InteractionRespondStateEnum),
          ) as InteractionRespondStateEnum;
          result.state = valueDes;
          break;
        case r'response':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(InteractionResponse),
          ) as InteractionResponse?;
          if (valueDes == null) continue;
          result.response.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InteractionRespond deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InteractionRespondBuilder();
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

class InteractionRespondStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'acknowledged')
  static const InteractionRespondStateEnum acknowledged =
      _$interactionRespondStateEnum_acknowledged;
  @BuiltValueEnumConst(wireName: r'failed')
  static const InteractionRespondStateEnum failed =
      _$interactionRespondStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'rejected')
  static const InteractionRespondStateEnum rejected =
      _$interactionRespondStateEnum_rejected;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const InteractionRespondStateEnum unknownDefaultOpenApi =
      _$interactionRespondStateEnum_unknownDefaultOpenApi;

  static Serializer<InteractionRespondStateEnum> get serializer =>
      _$interactionRespondStateEnumSerializer;

  const InteractionRespondStateEnum._(String name) : super(name);

  static BuiltSet<InteractionRespondStateEnum> get values =>
      _$interactionRespondStateEnumValues;
  static InteractionRespondStateEnum valueOf(String name) =>
      _$interactionRespondStateEnumValueOf(name);
}
