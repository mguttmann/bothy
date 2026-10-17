//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'instance_limits.g.dart';

/// Effective values of limits that clients enforce before sending.
///
/// Properties:
/// * [messageMaxContentBytes]
/// * [messageMaxAttachments]
/// * [fileMaxUploadBytes]
/// * [threadTitleMaxChars]
/// * [voiceMaxDurationMs] - Longest voice message (`voice.max_duration`, 12).
@BuiltValue()
abstract class InstanceLimits
    implements Built<InstanceLimits, InstanceLimitsBuilder> {
  @BuiltValueField(wireName: r'message_max_content_bytes')
  int get messageMaxContentBytes;

  @BuiltValueField(wireName: r'message_max_attachments')
  int get messageMaxAttachments;

  @BuiltValueField(wireName: r'file_max_upload_bytes')
  int get fileMaxUploadBytes;

  @BuiltValueField(wireName: r'thread_title_max_chars')
  int get threadTitleMaxChars;

  /// Longest voice message (`voice.max_duration`, 12).
  @BuiltValueField(wireName: r'voice_max_duration_ms')
  int get voiceMaxDurationMs;

  InstanceLimits._();

  factory InstanceLimits([void updates(InstanceLimitsBuilder b)]) =
      _$InstanceLimits;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InstanceLimitsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InstanceLimits> get serializer =>
      _$InstanceLimitsSerializer();
}

class _$InstanceLimitsSerializer
    implements PrimitiveSerializer<InstanceLimits> {
  @override
  final Iterable<Type> types = const [InstanceLimits, _$InstanceLimits];

  @override
  final String wireName = r'InstanceLimits';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InstanceLimits object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'message_max_content_bytes';
    yield serializers.serialize(
      object.messageMaxContentBytes,
      specifiedType: const FullType(int),
    );
    yield r'message_max_attachments';
    yield serializers.serialize(
      object.messageMaxAttachments,
      specifiedType: const FullType(int),
    );
    yield r'file_max_upload_bytes';
    yield serializers.serialize(
      object.fileMaxUploadBytes,
      specifiedType: const FullType(int),
    );
    yield r'thread_title_max_chars';
    yield serializers.serialize(
      object.threadTitleMaxChars,
      specifiedType: const FullType(int),
    );
    yield r'voice_max_duration_ms';
    yield serializers.serialize(
      object.voiceMaxDurationMs,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    InstanceLimits object, {
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
    required InstanceLimitsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'message_max_content_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.messageMaxContentBytes = valueDes;
          break;
        case r'message_max_attachments':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.messageMaxAttachments = valueDes;
          break;
        case r'file_max_upload_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.fileMaxUploadBytes = valueDes;
          break;
        case r'thread_title_max_chars':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.threadTitleMaxChars = valueDes;
          break;
        case r'voice_max_duration_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.voiceMaxDurationMs = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InstanceLimits deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InstanceLimitsBuilder();
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
