//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_set.g.dart';

/// HostSet
///
/// Properties:
/// * [hostedBy] - The new hosting runner, with the rules of `AgentCreate.hosted_by`.
/// * [reason]
@BuiltValue()
abstract class HostSet implements Built<HostSet, HostSetBuilder> {
  /// The new hosting runner, with the rules of `AgentCreate.hosted_by`.
  @BuiltValueField(wireName: r'hosted_by')
  String get hostedBy;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  HostSet._();

  factory HostSet([void updates(HostSetBuilder b)]) = _$HostSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostSet> get serializer => _$HostSetSerializer();
}

class _$HostSetSerializer implements PrimitiveSerializer<HostSet> {
  @override
  final Iterable<Type> types = const [HostSet, _$HostSet];

  @override
  final String wireName = r'HostSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'hosted_by';
    yield serializers.serialize(
      object.hostedBy,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostSet object, {
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
    required HostSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'hosted_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostedBy = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostSetBuilder();
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
