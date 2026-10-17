//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_passkey_request.g.dart';

/// StepUpPasskeyRequest
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [credential] - Free-form JSON object whose schema is named in the field description.
@BuiltValue()
abstract class StepUpPasskeyRequest
    implements Built<StepUpPasskeyRequest, StepUpPasskeyRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// Free-form JSON object whose schema is named in the field description.
  @BuiltValueField(wireName: r'credential')
  BuiltMap<String, JsonObject?> get credential;

  StepUpPasskeyRequest._();

  factory StepUpPasskeyRequest([void updates(StepUpPasskeyRequestBuilder b)]) =
      _$StepUpPasskeyRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpPasskeyRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpPasskeyRequest> get serializer =>
      _$StepUpPasskeyRequestSerializer();
}

class _$StepUpPasskeyRequestSerializer
    implements PrimitiveSerializer<StepUpPasskeyRequest> {
  @override
  final Iterable<Type> types = const [
    StepUpPasskeyRequest,
    _$StepUpPasskeyRequest,
  ];

  @override
  final String wireName = r'StepUpPasskeyRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpPasskeyRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'credential';
    yield serializers.serialize(
      object.credential,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpPasskeyRequest object, {
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
    required StepUpPasskeyRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'challenge_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.challengeId = valueDes;
          break;
        case r'credential':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.credential.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpPasskeyRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpPasskeyRequestBuilder();
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
