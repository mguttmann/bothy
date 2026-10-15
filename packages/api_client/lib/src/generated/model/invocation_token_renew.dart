//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'invocation_token_renew.g.dart';

/// InvocationTokenRenew
///
/// Properties:
/// * [invocationToken] - The current invocation token of the run, or on a retry after a lost answer the token the last renewal replaced; it may have expired (09 section 4.5.1).
@BuiltValue()
abstract class InvocationTokenRenew
    implements Built<InvocationTokenRenew, InvocationTokenRenewBuilder> {
  /// The current invocation token of the run, or on a retry after a lost answer the token the last renewal replaced; it may have expired (09 section 4.5.1).
  @BuiltValueField(wireName: r'invocation_token')
  String get invocationToken;

  InvocationTokenRenew._();

  factory InvocationTokenRenew([void updates(InvocationTokenRenewBuilder b)]) =
      _$InvocationTokenRenew;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InvocationTokenRenewBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InvocationTokenRenew> get serializer =>
      _$InvocationTokenRenewSerializer();
}

class _$InvocationTokenRenewSerializer
    implements PrimitiveSerializer<InvocationTokenRenew> {
  @override
  final Iterable<Type> types = const [
    InvocationTokenRenew,
    _$InvocationTokenRenew,
  ];

  @override
  final String wireName = r'InvocationTokenRenew';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InvocationTokenRenew object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'invocation_token';
    yield serializers.serialize(
      object.invocationToken,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    InvocationTokenRenew object, {
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
    required InvocationTokenRenewBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'invocation_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.invocationToken = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  InvocationTokenRenew deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InvocationTokenRenewBuilder();
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
