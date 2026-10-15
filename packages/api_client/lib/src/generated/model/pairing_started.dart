//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pairing_started.g.dart';

/// PairingStarted
///
/// Properties:
/// * [pairingId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [pairingCode] - Shown on the new device only; `auth.pairing_code_chars` characters (Crockford Base32).
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class PairingStarted
    implements Built<PairingStarted, PairingStartedBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'pairing_id')
  String get pairingId;

  /// Shown on the new device only; `auth.pairing_code_chars` characters (Crockford Base32).
  @BuiltValueField(wireName: r'pairing_code')
  String get pairingCode;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  PairingStarted._();

  factory PairingStarted([void updates(PairingStartedBuilder b)]) =
      _$PairingStarted;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PairingStartedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PairingStarted> get serializer =>
      _$PairingStartedSerializer();
}

class _$PairingStartedSerializer
    implements PrimitiveSerializer<PairingStarted> {
  @override
  final Iterable<Type> types = const [PairingStarted, _$PairingStarted];

  @override
  final String wireName = r'PairingStarted';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PairingStarted object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pairing_id';
    yield serializers.serialize(
      object.pairingId,
      specifiedType: const FullType(String),
    );
    yield r'pairing_code';
    yield serializers.serialize(
      object.pairingCode,
      specifiedType: const FullType(String),
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
    PairingStarted object, {
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
    required PairingStartedBuilder result,
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
        case r'pairing_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pairingCode = valueDes;
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
  PairingStarted deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PairingStartedBuilder();
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
