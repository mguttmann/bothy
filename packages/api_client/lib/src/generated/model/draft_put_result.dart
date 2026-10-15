//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/draft.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'draft_put_result.g.dart';

/// DraftPutResult
///
/// Properties:
/// * [draft]
/// * [overwroteNewer]
@BuiltValue()
abstract class DraftPutResult
    implements Built<DraftPutResult, DraftPutResultBuilder> {
  @BuiltValueField(wireName: r'draft')
  Draft get draft;

  @BuiltValueField(wireName: r'overwrote_newer')
  bool get overwroteNewer;

  DraftPutResult._();

  factory DraftPutResult([void updates(DraftPutResultBuilder b)]) =
      _$DraftPutResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DraftPutResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DraftPutResult> get serializer =>
      _$DraftPutResultSerializer();
}

class _$DraftPutResultSerializer
    implements PrimitiveSerializer<DraftPutResult> {
  @override
  final Iterable<Type> types = const [DraftPutResult, _$DraftPutResult];

  @override
  final String wireName = r'DraftPutResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DraftPutResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'draft';
    yield serializers.serialize(
      object.draft,
      specifiedType: const FullType(Draft),
    );
    yield r'overwrote_newer';
    yield serializers.serialize(
      object.overwroteNewer,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DraftPutResult object, {
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
    required DraftPutResultBuilder result,
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
        case r'overwrote_newer':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.overwroteNewer = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DraftPutResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DraftPutResultBuilder();
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
