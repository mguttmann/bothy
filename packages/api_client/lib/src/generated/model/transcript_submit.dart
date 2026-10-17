//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'transcript_submit.g.dart';

/// TranscriptSubmit
///
/// Properties:
/// * [text]
/// * [language]
@BuiltValue()
abstract class TranscriptSubmit
    implements Built<TranscriptSubmit, TranscriptSubmitBuilder> {
  @BuiltValueField(wireName: r'text')
  String get text;

  @BuiltValueField(wireName: r'language')
  String? get language;

  TranscriptSubmit._();

  factory TranscriptSubmit([void updates(TranscriptSubmitBuilder b)]) =
      _$TranscriptSubmit;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TranscriptSubmitBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TranscriptSubmit> get serializer =>
      _$TranscriptSubmitSerializer();
}

class _$TranscriptSubmitSerializer
    implements PrimitiveSerializer<TranscriptSubmit> {
  @override
  final Iterable<Type> types = const [TranscriptSubmit, _$TranscriptSubmit];

  @override
  final String wireName = r'TranscriptSubmit';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TranscriptSubmit object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'text';
    yield serializers.serialize(
      object.text,
      specifiedType: const FullType(String),
    );
    if (object.language != null) {
      yield r'language';
      yield serializers.serialize(
        object.language,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    TranscriptSubmit object, {
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
    required TranscriptSubmitBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.text = valueDes;
          break;
        case r'language':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.language = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TranscriptSubmit deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TranscriptSubmitBuilder();
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
