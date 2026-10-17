//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'client_metric_buckets_inner.g.dart';

/// ClientMetricBucketsInner
///
/// Properties:
/// * [le]
/// * [count]
@BuiltValue()
abstract class ClientMetricBucketsInner
    implements
        Built<ClientMetricBucketsInner, ClientMetricBucketsInnerBuilder> {
  @BuiltValueField(wireName: r'le')
  double get le;

  @BuiltValueField(wireName: r'count')
  int get count;

  ClientMetricBucketsInner._();

  factory ClientMetricBucketsInner([
    void updates(ClientMetricBucketsInnerBuilder b),
  ]) = _$ClientMetricBucketsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ClientMetricBucketsInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ClientMetricBucketsInner> get serializer =>
      _$ClientMetricBucketsInnerSerializer();
}

class _$ClientMetricBucketsInnerSerializer
    implements PrimitiveSerializer<ClientMetricBucketsInner> {
  @override
  final Iterable<Type> types = const [
    ClientMetricBucketsInner,
    _$ClientMetricBucketsInner,
  ];

  @override
  final String wireName = r'ClientMetricBucketsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ClientMetricBucketsInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'le';
    yield serializers.serialize(
      object.le,
      specifiedType: const FullType(double),
    );
    yield r'count';
    yield serializers.serialize(
      object.count,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricBucketsInner object, {
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
    required ClientMetricBucketsInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'le':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(double),
          ) as double;
          result.le = valueDes;
          break;
        case r'count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.count = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ClientMetricBucketsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ClientMetricBucketsInnerBuilder();
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
