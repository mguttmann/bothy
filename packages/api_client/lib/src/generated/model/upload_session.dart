//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/upload_part.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_session.g.dart';

/// Upload plan. In mode `direct` the part URLs point to this server; in mode `presigned` they point to the object store. The client uploads all parts, then calls complete.
///
/// Properties:
/// * [fileId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [mode]
/// * [partSizeBytes]
/// * [parts]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class UploadSession
    implements Built<UploadSession, UploadSessionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'file_id')
  String get fileId;

  @BuiltValueField(wireName: r'mode')
  UploadSessionModeEnum get mode;
  // enum modeEnum {  direct,  presigned,  };

  @BuiltValueField(wireName: r'part_size_bytes')
  int get partSizeBytes;

  @BuiltValueField(wireName: r'parts')
  BuiltList<UploadPart> get parts;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  UploadSession._();

  factory UploadSession([void updates(UploadSessionBuilder b)]) =
      _$UploadSession;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadSessionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadSession> get serializer =>
      _$UploadSessionSerializer();
}

class _$UploadSessionSerializer implements PrimitiveSerializer<UploadSession> {
  @override
  final Iterable<Type> types = const [UploadSession, _$UploadSession];

  @override
  final String wireName = r'UploadSession';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadSession object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'file_id';
    yield serializers.serialize(
      object.fileId,
      specifiedType: const FullType(String),
    );
    yield r'mode';
    yield serializers.serialize(
      object.mode,
      specifiedType: const FullType(UploadSessionModeEnum),
    );
    yield r'part_size_bytes';
    yield serializers.serialize(
      object.partSizeBytes,
      specifiedType: const FullType(int),
    );
    yield r'parts';
    yield serializers.serialize(
      object.parts,
      specifiedType: const FullType(BuiltList, [FullType(UploadPart)]),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    UploadSession object, {
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
    required UploadSessionBuilder result,
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
        case r'mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(UploadSessionModeEnum),
          ) as UploadSessionModeEnum;
          result.mode = valueDes;
          break;
        case r'part_size_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.partSizeBytes = valueDes;
          break;
        case r'parts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(UploadPart)]),
          ) as BuiltList<UploadPart>;
          result.parts.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UploadSession deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadSessionBuilder();
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

class UploadSessionModeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'direct')
  static const UploadSessionModeEnum direct = _$uploadSessionModeEnum_direct;
  @BuiltValueEnumConst(wireName: r'presigned')
  static const UploadSessionModeEnum presigned =
      _$uploadSessionModeEnum_presigned;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const UploadSessionModeEnum unknownDefaultOpenApi =
      _$uploadSessionModeEnum_unknownDefaultOpenApi;

  static Serializer<UploadSessionModeEnum> get serializer =>
      _$uploadSessionModeEnumSerializer;

  const UploadSessionModeEnum._(String name) : super(name);

  static BuiltSet<UploadSessionModeEnum> get values =>
      _$uploadSessionModeEnumValues;
  static UploadSessionModeEnum valueOf(String name) =>
      _$uploadSessionModeEnumValueOf(name);
}
