//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/draft.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'draft_payload.g.dart';

/// DraftPayload
///
/// Properties:
/// * [draft]
@BuiltValue()
abstract class DraftPayload
    implements Built<DraftPayload, DraftPayloadBuilder> {
  @BuiltValueField(wireName: r'draft')
  Draft get draft;

  DraftPayload._();

  factory DraftPayload([void updates(DraftPayloadBuilder b)]) = _$DraftPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DraftPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DraftPayload> get serializer => _$DraftPayloadSerializer();
}

class _$DraftPayloadSerializer implements PrimitiveSerializer<DraftPayload> {
  @override
  final Iterable<Type> types = const [DraftPayload, _$DraftPayload];

  @override
  final String wireName = r'DraftPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DraftPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'draft';
    yield serializers.serialize(
      object.draft,
      specifiedType: const FullType(Draft),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DraftPayload object, {
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
    required DraftPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'draft':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Draft),
          ) as Draft;
          result.draft.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DraftPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DraftPayloadBuilder();
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
