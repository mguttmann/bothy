//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pairing_status.g.dart';

/// PairingStatus
///
/// Properties:
/// * [pairingId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [status] - Not yet approved; ask again after a short wait.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class PairingStatus
    implements Built<PairingStatus, PairingStatusBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'pairing_id')
  String get pairingId;

  /// Not yet approved; ask again after a short wait.
  @BuiltValueField(wireName: r'status')
  PairingStatusStatusEnum get status;
  // enum statusEnum {  requested,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  PairingStatus._();

  factory PairingStatus([void updates(PairingStatusBuilder b)]) =
      _$PairingStatus;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PairingStatusBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PairingStatus> get serializer =>
      _$PairingStatusSerializer();
}

class _$PairingStatusSerializer implements PrimitiveSerializer<PairingStatus> {
  @override
  final Iterable<Type> types = const [PairingStatus, _$PairingStatus];

  @override
  final String wireName = r'PairingStatus';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PairingStatus object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pairing_id';
    yield serializers.serialize(
      object.pairingId,
      specifiedType: const FullType(String),
    );
    yield r'status';
    yield serializers.serialize(
      object.status,
      specifiedType: const FullType(PairingStatusStatusEnum),
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
    PairingStatus object, {
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
    required PairingStatusBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pairing_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pairingId = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PairingStatusStatusEnum),
          ) as PairingStatusStatusEnum;
          result.status = valueDes;
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
  PairingStatus deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PairingStatusBuilder();
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

/// Not yet approved; ask again after a short wait.
class PairingStatusStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'requested')
  static const PairingStatusStatusEnum requested =
      _$pairingStatusStatusEnum_requested;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const PairingStatusStatusEnum unknownDefaultOpenApi =
      _$pairingStatusStatusEnum_unknownDefaultOpenApi;

  static Serializer<PairingStatusStatusEnum> get serializer =>
      _$pairingStatusStatusEnumSerializer;

  const PairingStatusStatusEnum._(String name) : super(name);

  static BuiltSet<PairingStatusStatusEnum> get values =>
      _$pairingStatusStatusEnumValues;
  static PairingStatusStatusEnum valueOf(String name) =>
      _$pairingStatusStatusEnumValueOf(name);
}
