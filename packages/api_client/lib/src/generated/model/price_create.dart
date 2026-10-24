//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/price_unit.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'price_create.g.dart';

/// A price per unit (per million tokens for the `*_mtok` units) with its public source. `valid_to`, when present, lies after `valid_from`; `valid_from` may lie in the past. `source_url` is an absolute https URL without user info and fragment; `retrieved_at` is not in the future.
///
/// Properties:
/// * [provider]
/// * [model]
/// * [unit]
/// * [priceMicros]
/// * [currency]
/// * [validFrom] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [validTo] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [sourceUrl]
/// * [retrievedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class PriceCreate implements Built<PriceCreate, PriceCreateBuilder> {
  @BuiltValueField(wireName: r'provider')
  String get provider;

  @BuiltValueField(wireName: r'model')
  String get model;

  @BuiltValueField(wireName: r'unit')
  PriceUnit get unit;
  // enum unitEnum {  input_mtok,  output_mtok,  cache_read_mtok,  cache_write_mtok,  cache_write_1h_mtok,  request,  };

  @BuiltValueField(wireName: r'price_micros')
  int get priceMicros;

  @BuiltValueField(wireName: r'currency')
  String get currency;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'valid_from')
  DateTime get validFrom;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'valid_to')
  DateTime? get validTo;

  @BuiltValueField(wireName: r'source_url')
  String get sourceUrl;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'retrieved_at')
  DateTime get retrievedAt;

  PriceCreate._();

  factory PriceCreate([void updates(PriceCreateBuilder b)]) = _$PriceCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PriceCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PriceCreate> get serializer => _$PriceCreateSerializer();
}

class _$PriceCreateSerializer implements PrimitiveSerializer<PriceCreate> {
  @override
  final Iterable<Type> types = const [PriceCreate, _$PriceCreate];

  @override
  final String wireName = r'PriceCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PriceCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(String),
    );
    yield r'model';
    yield serializers.serialize(
      object.model,
      specifiedType: const FullType(String),
    );
    yield r'unit';
    yield serializers.serialize(
      object.unit,
      specifiedType: const FullType(PriceUnit),
    );
    yield r'price_micros';
    yield serializers.serialize(
      object.priceMicros,
      specifiedType: const FullType(int),
    );
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
    yield r'valid_from';
    yield serializers.serialize(
      object.validFrom,
      specifiedType: const FullType(DateTime),
    );
    if (object.validTo != null) {
      yield r'valid_to';
      yield serializers.serialize(
        object.validTo,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'source_url';
    yield serializers.serialize(
      object.sourceUrl,
      specifiedType: const FullType(String),
    );
    yield r'retrieved_at';
    yield serializers.serialize(
      object.retrievedAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PriceCreate object, {
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
    required PriceCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.provider = valueDes;
          break;
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.model = valueDes;
          break;
        case r'unit':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PriceUnit),
          ) as PriceUnit;
          result.unit = valueDes;
          break;
        case r'price_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.priceMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        case r'valid_from':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.validFrom = valueDes;
          break;
        case r'valid_to':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.validTo = valueDes;
          break;
        case r'source_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sourceUrl = valueDes;
          break;
        case r'retrieved_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.retrievedAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PriceCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PriceCreateBuilder();
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
