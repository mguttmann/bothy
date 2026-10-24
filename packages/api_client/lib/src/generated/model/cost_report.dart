//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_report.g.dart';

/// Cost of one model in this run as reported by the adapter: the consumption of this run, never a session sum; stored as estimate. The server computes the amount from the price table when the price set of provider, model and the end of the run covers every consumed unit (source `computed`, the adapter amount is ignored), otherwise it books `amount_micros` (source `adapter_reported`), otherwise 0 (source `unpriced`, \"Price missing\"). Token counts and `amount_micros` are not negative and below 2^53; absent token counts are 0.
///
/// Properties:
/// * [provider]
/// * [model]
/// * [tokensIn]
/// * [tokensOut]
/// * [tokensCacheRead]
/// * [tokensCacheWrite] - Cache writes with 5 minutes (or without a stated duration).
/// * [tokensCacheWrite1h] - Cache writes with 1 hour, priced with `cache_write_1h_mtok`.
/// * [amountMicros] - Absent lets the server compute from the price table.
/// * [currency] - ISO 4217 code.
@BuiltValue()
abstract class CostReport implements Built<CostReport, CostReportBuilder> {
  @BuiltValueField(wireName: r'provider')
  String get provider;

  @BuiltValueField(wireName: r'model')
  String get model;

  @BuiltValueField(wireName: r'tokens_in')
  int? get tokensIn;

  @BuiltValueField(wireName: r'tokens_out')
  int? get tokensOut;

  @BuiltValueField(wireName: r'tokens_cache_read')
  int? get tokensCacheRead;

  /// Cache writes with 5 minutes (or without a stated duration).
  @BuiltValueField(wireName: r'tokens_cache_write')
  int? get tokensCacheWrite;

  /// Cache writes with 1 hour, priced with `cache_write_1h_mtok`.
  @BuiltValueField(wireName: r'tokens_cache_write_1h')
  int? get tokensCacheWrite1h;

  /// Absent lets the server compute from the price table.
  @BuiltValueField(wireName: r'amount_micros')
  int? get amountMicros;

  /// ISO 4217 code.
  @BuiltValueField(wireName: r'currency')
  String get currency;

  CostReport._();

  factory CostReport([void updates(CostReportBuilder b)]) = _$CostReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostReport> get serializer => _$CostReportSerializer();
}

class _$CostReportSerializer implements PrimitiveSerializer<CostReport> {
  @override
  final Iterable<Type> types = const [CostReport, _$CostReport];

  @override
  final String wireName = r'CostReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostReport object, {
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
    if (object.tokensIn != null) {
      yield r'tokens_in';
      yield serializers.serialize(
        object.tokensIn,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensOut != null) {
      yield r'tokens_out';
      yield serializers.serialize(
        object.tokensOut,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheRead != null) {
      yield r'tokens_cache_read';
      yield serializers.serialize(
        object.tokensCacheRead,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheWrite != null) {
      yield r'tokens_cache_write';
      yield serializers.serialize(
        object.tokensCacheWrite,
        specifiedType: const FullType(int),
      );
    }
    if (object.tokensCacheWrite1h != null) {
      yield r'tokens_cache_write_1h';
      yield serializers.serialize(
        object.tokensCacheWrite1h,
        specifiedType: const FullType(int),
      );
    }
    if (object.amountMicros != null) {
      yield r'amount_micros';
      yield serializers.serialize(
        object.amountMicros,
        specifiedType: const FullType(int),
      );
    }
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostReport object, {
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
    required CostReportBuilder result,
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
        case r'tokens_in':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensIn = valueDes;
          break;
        case r'tokens_out':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensOut = valueDes;
          break;
        case r'tokens_cache_read':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheRead = valueDes;
          break;
        case r'tokens_cache_write':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheWrite = valueDes;
          break;
        case r'tokens_cache_write_1h':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.tokensCacheWrite1h = valueDes;
          break;
        case r'amount_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.amountMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostReportBuilder();
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
