//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/transcript_status.dart';
import 'package:api_client/src/generated/model/transcript_source.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'transcript.g.dart';

/// Transcript
///
/// Properties:
/// * [fileId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [status]
/// * [text]
/// * [language]
/// * [source_]
@BuiltValue()
abstract class Transcript implements Built<Transcript, TranscriptBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'file_id')
  String get fileId;

  @BuiltValueField(wireName: r'status')
  TranscriptStatus get status;
  // enum statusEnum {  none,  pending,  done,  failed,  };

  @BuiltValueField(wireName: r'text')
  String? get text;

  @BuiltValueField(wireName: r'language')
  String? get language;

  @BuiltValueField(wireName: r'source')
  TranscriptSource? get source_;
  // enum source_Enum {  agent,  service,  };

  Transcript._();

  factory Transcript([void updates(TranscriptBuilder b)]) = _$Transcript;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TranscriptBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Transcript> get serializer => _$TranscriptSerializer();
}

class _$TranscriptSerializer implements PrimitiveSerializer<Transcript> {
  @override
  final Iterable<Type> types = const [Transcript, _$Transcript];

  @override
  final String wireName = r'Transcript';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Transcript object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'file_id';
    yield serializers.serialize(
      object.fileId,
      specifiedType: const FullType(String),
    );
    yield r'status';
    yield serializers.serialize(
      object.status,
      specifiedType: const FullType(TranscriptStatus),
    );
    if (object.text != null) {
      yield r'text';
      yield serializers.serialize(
        object.text,
        specifiedType: const FullType(String),
      );
    }
    if (object.language != null) {
      yield r'language';
      yield serializers.serialize(
        object.language,
        specifiedType: const FullType(String),
      );
    }
    if (object.source_ != null) {
      yield r'source';
      yield serializers.serialize(
        object.source_,
        specifiedType: const FullType(TranscriptSource),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Transcript object, {
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
    required TranscriptBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'file_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.fileId = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TranscriptStatus),
          ) as TranscriptStatus;
          result.status = valueDes;
          break;
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
        case r'source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TranscriptSource),
          ) as TranscriptSource?;
          if (valueDes == null) continue;
          result.source_ = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Transcript deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TranscriptBuilder();
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
