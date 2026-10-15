//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_revision.g.dart';

/// MessageRevision
///
/// Properties:
/// * [revision]
/// * [contentMd]
/// * [editedBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class MessageRevision
    implements Built<MessageRevision, MessageRevisionBuilder> {
  @BuiltValueField(wireName: r'revision')
  int get revision;

  @BuiltValueField(wireName: r'content_md')
  String get contentMd;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'edited_by')
  String get editedBy;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  MessageRevision._();

  factory MessageRevision([void updates(MessageRevisionBuilder b)]) =
      _$MessageRevision;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MessageRevisionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<MessageRevision> get serializer =>
      _$MessageRevisionSerializer();
}

class _$MessageRevisionSerializer
    implements PrimitiveSerializer<MessageRevision> {
  @override
  final Iterable<Type> types = const [MessageRevision, _$MessageRevision];

  @override
  final String wireName = r'MessageRevision';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    MessageRevision object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'revision';
    yield serializers.serialize(
      object.revision,
      specifiedType: const FullType(int),
    );
    yield r'content_md';
    yield serializers.serialize(
      object.contentMd,
      specifiedType: const FullType(String),
    );
    yield r'edited_by';
    yield serializers.serialize(
      object.editedBy,
      specifiedType: const FullType(String),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    MessageRevision object, {
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
    required MessageRevisionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'revision':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.revision = valueDes;
          break;
        case r'content_md':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.contentMd = valueDes;
          break;
        case r'edited_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.editedBy = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  MessageRevision deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MessageRevisionBuilder();
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
