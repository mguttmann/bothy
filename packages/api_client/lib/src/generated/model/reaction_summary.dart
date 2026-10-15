//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'reaction_summary.g.dart';

/// ReactionSummary
///
/// Properties:
/// * [emoji] - Exactly one Unicode grapheme cluster.
/// * [count]
@BuiltValue()
abstract class ReactionSummary
    implements Built<ReactionSummary, ReactionSummaryBuilder> {
  /// Exactly one Unicode grapheme cluster.
  @BuiltValueField(wireName: r'emoji')
  String get emoji;

  @BuiltValueField(wireName: r'count')
  int get count;

  ReactionSummary._();

  factory ReactionSummary([void updates(ReactionSummaryBuilder b)]) =
      _$ReactionSummary;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ReactionSummaryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ReactionSummary> get serializer =>
      _$ReactionSummarySerializer();
}

class _$ReactionSummarySerializer
    implements PrimitiveSerializer<ReactionSummary> {
  @override
  final Iterable<Type> types = const [ReactionSummary, _$ReactionSummary];

  @override
  final String wireName = r'ReactionSummary';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ReactionSummary object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'emoji';
    yield serializers.serialize(
      object.emoji,
      specifiedType: const FullType(String),
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
    ReactionSummary object, {
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
    required ReactionSummaryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'emoji':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.emoji = valueDes;
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
  ReactionSummary deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ReactionSummaryBuilder();
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
