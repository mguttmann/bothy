//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'token_counts.g.dart';

/// TokenCounts
///
/// Properties:
/// * [tokensIn]
/// * [tokensOut]
/// * [tokensCacheRead]
/// * [tokensCacheWrite]
/// * [tokensCacheWrite1h]
@BuiltValue()
abstract class TokenCounts implements Built<TokenCounts, TokenCountsBuilder> {
  @BuiltValueField(wireName: r'tokens_in')
  int get tokensIn;

  @BuiltValueField(wireName: r'tokens_out')
  int get tokensOut;

  @BuiltValueField(wireName: r'tokens_cache_read')
  int get tokensCacheRead;

  @BuiltValueField(wireName: r'tokens_cache_write')
  int get tokensCacheWrite;

  @BuiltValueField(wireName: r'tokens_cache_write_1h')
  int get tokensCacheWrite1h;

  TokenCounts._();

  factory TokenCounts([void updates(TokenCountsBuilder b)]) = _$TokenCounts;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TokenCountsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TokenCounts> get serializer => _$TokenCountsSerializer();
}

class _$TokenCountsSerializer implements PrimitiveSerializer<TokenCounts> {
  @override
  final Iterable<Type> types = const [TokenCounts, _$TokenCounts];

  @override
  final String wireName = r'TokenCounts';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TokenCounts object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'tokens_in';
    yield serializers.serialize(
      object.tokensIn,
      specifiedType: const FullType(int),
    );
    yield r'tokens_out';
    yield serializers.serialize(
      object.tokensOut,
      specifiedType: const FullType(int),
    );
    yield r'tokens_cache_read';
    yield serializers.serialize(
      object.tokensCacheRead,
      specifiedType: const FullType(int),
    );
    yield r'tokens_cache_write';
    yield serializers.serialize(
      object.tokensCacheWrite,
      specifiedType: const FullType(int),
    );
    yield r'tokens_cache_write_1h';
    yield serializers.serialize(
      object.tokensCacheWrite1h,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TokenCounts object, {
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
    required TokenCountsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'tokens_in':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.tokensIn = valueDes;
          break;
        case r'tokens_out':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.tokensOut = valueDes;
          break;
        case r'tokens_cache_read':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.tokensCacheRead = valueDes;
          break;
        case r'tokens_cache_write':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.tokensCacheWrite = valueDes;
          break;
        case r'tokens_cache_write_1h':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.tokensCacheWrite1h = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TokenCounts deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TokenCountsBuilder();
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
