//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'import_file_create.g.dart';

/// ImportFileCreate
///
/// Properties:
/// * [originKey]
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [authorId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [sizeBytes]
/// * [sha256] - Lowercase hex SHA-256 of the bytes (`file_upload_sessions.declared_sha256`); the scan checks the bytes against it.
/// * [declaredMimeType]
@BuiltValue()
abstract class ImportFileCreate
    implements Built<ImportFileCreate, ImportFileCreateBuilder> {
  @BuiltValueField(wireName: r'origin_key')
  String get originKey;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'author_id')
  String get authorId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'size_bytes')
  int get sizeBytes;

  /// Lowercase hex SHA-256 of the bytes (`file_upload_sessions.declared_sha256`); the scan checks the bytes against it.
  @BuiltValueField(wireName: r'sha256')
  String get sha256;

  @BuiltValueField(wireName: r'declared_mime_type')
  String? get declaredMimeType;

  ImportFileCreate._();

  factory ImportFileCreate([void updates(ImportFileCreateBuilder b)]) =
      _$ImportFileCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ImportFileCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ImportFileCreate> get serializer =>
      _$ImportFileCreateSerializer();
}

class _$ImportFileCreateSerializer
    implements PrimitiveSerializer<ImportFileCreate> {
  @override
  final Iterable<Type> types = const [ImportFileCreate, _$ImportFileCreate];

  @override
  final String wireName = r'ImportFileCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ImportFileCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'origin_key';
    yield serializers.serialize(
      object.originKey,
      specifiedType: const FullType(String),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'author_id';
    yield serializers.serialize(
      object.authorId,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'size_bytes';
    yield serializers.serialize(
      object.sizeBytes,
      specifiedType: const FullType(int),
    );
    yield r'sha256';
    yield serializers.serialize(
      object.sha256,
      specifiedType: const FullType(String),
    );
    if (object.declaredMimeType != null) {
      yield r'declared_mime_type';
      yield serializers.serialize(
        object.declaredMimeType,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ImportFileCreate object, {
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
    required ImportFileCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'origin_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.originKey = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'author_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.authorId = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'size_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.sizeBytes = valueDes;
          break;
        case r'sha256':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sha256 = valueDes;
          break;
        case r'declared_mime_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.declaredMimeType = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ImportFileCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ImportFileCreateBuilder();
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
