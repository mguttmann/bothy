//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/gdpr_request_kind.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_request_due_payload.g.dart';

/// An open privacy request reaches the reminder lead before its statutory deadline (R-AZ-35). Carries only the ID, the kind and the deadline.
///
/// Properties:
/// * [requestId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [kind]
/// * [dueAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class GdprRequestDuePayload
    implements Built<GdprRequestDuePayload, GdprRequestDuePayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'request_id')
  String get requestId;

  @BuiltValueField(wireName: r'kind')
  GdprRequestKind get kind;
  // enum kindEnum {  export,  deletion,  rectification,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'due_at')
  DateTime get dueAt;

  GdprRequestDuePayload._();

  factory GdprRequestDuePayload([
    void updates(GdprRequestDuePayloadBuilder b),
  ]) = _$GdprRequestDuePayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GdprRequestDuePayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GdprRequestDuePayload> get serializer =>
      _$GdprRequestDuePayloadSerializer();
}

class _$GdprRequestDuePayloadSerializer
    implements PrimitiveSerializer<GdprRequestDuePayload> {
  @override
  final Iterable<Type> types = const [
    GdprRequestDuePayload,
    _$GdprRequestDuePayload,
  ];

  @override
  final String wireName = r'GdprRequestDuePayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GdprRequestDuePayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'request_id';
    yield serializers.serialize(
      object.requestId,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(GdprRequestKind),
    );
    yield r'due_at';
    yield serializers.serialize(
      object.dueAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GdprRequestDuePayload object, {
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
    required GdprRequestDuePayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'request_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.requestId = valueDes;
          break;
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(GdprRequestKind),
          ) as GdprRequestKind;
          result.kind = valueDes;
          break;
        case r'due_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.dueAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GdprRequestDuePayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GdprRequestDuePayloadBuilder();
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
