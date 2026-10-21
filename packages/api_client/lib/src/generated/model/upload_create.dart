//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_create.g.dart';

/// UploadCreate
///
/// Properties:
/// * [name]
/// * [sizeBytes]
/// * [declaredMimeType]
/// * [sha256] - Lowercase hex SHA-256 of the whole file; the verify pass checks it.
@BuiltValue()
abstract class UploadCreate
    implements Built<UploadCreate, UploadCreateBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'size_bytes')
  int get sizeBytes;

  @BuiltValueField(wireName: r'declared_mime_type')
  String? get declaredMimeType;

  /// Lowercase hex SHA-256 of the whole file; the verify pass checks it.
  @BuiltValueField(wireName: r'sha256')
  String get sha256;

  UploadCreate._();

  factory UploadCreate([void updates(UploadCreateBuilder b)]) = _$UploadCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadCreate> get serializer => _$UploadCreateSerializer();
}

class _$UploadCreateSerializer implements PrimitiveSerializer<UploadCreate> {
  @override
  final Iterable<Type> types = const [UploadCreate, _$UploadCreate];

  @override
  final String wireName = r'UploadCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    if (object.declaredMimeType != null) {
      yield r'declared_mime_type';
      yield serializers.serialize(
        object.declaredMimeType,
        specifiedType: const FullType(String),
      );
    }
    yield r'sha256';
    yield serializers.serialize(
      object.sha256,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UploadCreate object, {
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
    required UploadCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
        case r'declared_mime_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.declaredMimeType = valueDes;
          break;
        case r'sha256':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sha256 = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UploadCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadCreateBuilder();
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
