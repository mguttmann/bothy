//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/upload_state_completed_parts_inner.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'upload_state.g.dart';

/// State of an own upload session, for resuming after a network loss.
///
/// Properties:
/// * [fileId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [uploadState]
/// * [partSizeBytes]
/// * [completedParts] - Parts the server has confirmed, with their ETag.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class UploadState implements Built<UploadState, UploadStateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'file_id')
  String get fileId;

  @BuiltValueField(wireName: r'upload_state')
  UploadStateUploadStateEnum get uploadState;
  // enum uploadStateEnum {  pending,  uploading,  complete,  };

  @BuiltValueField(wireName: r'part_size_bytes')
  int get partSizeBytes;

  /// Parts the server has confirmed, with their ETag.
  @BuiltValueField(wireName: r'completed_parts')
  BuiltList<UploadStateCompletedPartsInner> get completedParts;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  UploadState._();

  factory UploadState([void updates(UploadStateBuilder b)]) = _$UploadState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UploadStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UploadState> get serializer => _$UploadStateSerializer();
}

class _$UploadStateSerializer implements PrimitiveSerializer<UploadState> {
  @override
  final Iterable<Type> types = const [UploadState, _$UploadState];

  @override
  final String wireName = r'UploadState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UploadState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'file_id';
    yield serializers.serialize(
      object.fileId,
      specifiedType: const FullType(String),
    );
    yield r'upload_state';
    yield serializers.serialize(
      object.uploadState,
      specifiedType: const FullType(UploadStateUploadStateEnum),
    );
    yield r'part_size_bytes';
    yield serializers.serialize(
      object.partSizeBytes,
      specifiedType: const FullType(int),
    );
    yield r'completed_parts';
    yield serializers.serialize(
      object.completedParts,
      specifiedType: const FullType(BuiltList, [
        FullType(UploadStateCompletedPartsInner),
      ]),
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
    UploadState object, {
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
    required UploadStateBuilder result,
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
        case r'upload_state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(UploadStateUploadStateEnum),
          ) as UploadStateUploadStateEnum;
          result.uploadState = valueDes;
          break;
        case r'part_size_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.partSizeBytes = valueDes;
          break;
        case r'completed_parts':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(UploadStateCompletedPartsInner),
            ]),
          ) as BuiltList<UploadStateCompletedPartsInner>;
          result.completedParts.replace(valueDes);
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
  UploadState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UploadStateBuilder();
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

class UploadStateUploadStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const UploadStateUploadStateEnum pending =
      _$uploadStateUploadStateEnum_pending;
  @BuiltValueEnumConst(wireName: r'uploading')
  static const UploadStateUploadStateEnum uploading =
      _$uploadStateUploadStateEnum_uploading;
  @BuiltValueEnumConst(wireName: r'complete')
  static const UploadStateUploadStateEnum complete =
      _$uploadStateUploadStateEnum_complete;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const UploadStateUploadStateEnum unknownDefaultOpenApi =
      _$uploadStateUploadStateEnum_unknownDefaultOpenApi;

  static Serializer<UploadStateUploadStateEnum> get serializer =>
      _$uploadStateUploadStateEnumSerializer;

  const UploadStateUploadStateEnum._(String name) : super(name);

  static BuiltSet<UploadStateUploadStateEnum> get values =>
      _$uploadStateUploadStateEnumValues;
  static UploadStateUploadStateEnum valueOf(String name) =>
      _$uploadStateUploadStateEnumValueOf(name);
}
