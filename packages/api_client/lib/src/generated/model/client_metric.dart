//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/client_metric_buckets_inner.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'client_metric.g.dart';

/// ClientMetric
///
/// Properties:
/// * [name] - One name of the closed list in docs/observability/signals.md section 4.6a: `client.cold_start.duration`, `client.thread_open.duration`, `client.send_local.duration`, `client.frames`, `client.frames.over_budget`, `client.catchup.duration`, `client.catchup.events`, `client.resync`, `client.reconnects`, `client.outbox.depth`, `client.push_display.duration`, `client.push.placeholder`. Not an enum on purpose: the server drops an entry with an unknown name, or with a `kind` that does not match the type of its name, one by one and still accepts the rest of the batch, so a newer app does not lose its batch against an older server.
/// * [kind]
/// * [closeCode] - Only for `client.reconnects`; a close code of 04 section 6.4.
/// * [value] - Counter increment or gauge value.
/// * [buckets] - Histogram buckets (upper bound and count).
@BuiltValue()
abstract class ClientMetric
    implements Built<ClientMetric, ClientMetricBuilder> {
  /// One name of the closed list in docs/observability/signals.md section 4.6a: `client.cold_start.duration`, `client.thread_open.duration`, `client.send_local.duration`, `client.frames`, `client.frames.over_budget`, `client.catchup.duration`, `client.catchup.events`, `client.resync`, `client.reconnects`, `client.outbox.depth`, `client.push_display.duration`, `client.push.placeholder`. Not an enum on purpose: the server drops an entry with an unknown name, or with a `kind` that does not match the type of its name, one by one and still accepts the rest of the batch, so a newer app does not lose its batch against an older server.
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'kind')
  ClientMetricKindEnum get kind;
  // enum kindEnum {  histogram,  counter,  gauge,  };

  /// Only for `client.reconnects`; a close code of 04 section 6.4.
  @BuiltValueField(wireName: r'close_code')
  int? get closeCode;

  /// Counter increment or gauge value.
  @BuiltValueField(wireName: r'value')
  double? get value;

  /// Histogram buckets (upper bound and count).
  @BuiltValueField(wireName: r'buckets')
  BuiltList<ClientMetricBucketsInner>? get buckets;

  ClientMetric._();

  factory ClientMetric([void updates(ClientMetricBuilder b)]) = _$ClientMetric;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ClientMetricBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ClientMetric> get serializer => _$ClientMetricSerializer();
}

class _$ClientMetricSerializer implements PrimitiveSerializer<ClientMetric> {
  @override
  final Iterable<Type> types = const [ClientMetric, _$ClientMetric];

  @override
  final String wireName = r'ClientMetric';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ClientMetric object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(ClientMetricKindEnum),
    );
    if (object.closeCode != null) {
      yield r'close_code';
      yield serializers.serialize(
        object.closeCode,
        specifiedType: const FullType(int),
      );
    }
    if (object.value != null) {
      yield r'value';
      yield serializers.serialize(
        object.value,
        specifiedType: const FullType(double),
      );
    }
    if (object.buckets != null) {
      yield r'buckets';
      yield serializers.serialize(
        object.buckets,
        specifiedType: const FullType(BuiltList, [
          FullType(ClientMetricBucketsInner),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ClientMetric object, {
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
    required ClientMetricBuilder result,
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
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ClientMetricKindEnum),
          ) as ClientMetricKindEnum;
          result.kind = valueDes;
          break;
        case r'close_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.closeCode = valueDes;
          break;
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.value = valueDes;
          break;
        case r'buckets':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(ClientMetricBucketsInner),
            ]),
          ) as BuiltList<ClientMetricBucketsInner>?;
          if (valueDes == null) continue;
          result.buckets.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ClientMetric deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ClientMetricBuilder();
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

class ClientMetricKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'histogram')
  static const ClientMetricKindEnum histogram =
      _$clientMetricKindEnum_histogram;
  @BuiltValueEnumConst(wireName: r'counter')
  static const ClientMetricKindEnum counter = _$clientMetricKindEnum_counter;
  @BuiltValueEnumConst(wireName: r'gauge')
  static const ClientMetricKindEnum gauge = _$clientMetricKindEnum_gauge;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ClientMetricKindEnum unknownDefaultOpenApi =
      _$clientMetricKindEnum_unknownDefaultOpenApi;

  static Serializer<ClientMetricKindEnum> get serializer =>
      _$clientMetricKindEnumSerializer;

  const ClientMetricKindEnum._(String name) : super(name);

  static BuiltSet<ClientMetricKindEnum> get values =>
      _$clientMetricKindEnumValues;
  static ClientMetricKindEnum valueOf(String name) =>
      _$clientMetricKindEnumValueOf(name);
}
