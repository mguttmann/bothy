//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_read_only_set.g.dart';

/// ChannelReadOnlySet
///
/// Properties:
/// * [readOnly]
/// * [reason]
@BuiltValue()
abstract class ChannelReadOnlySet
    implements Built<ChannelReadOnlySet, ChannelReadOnlySetBuilder> {
  @BuiltValueField(wireName: r'read_only')
  bool get readOnly;

  @BuiltValueField(wireName: r'reason')
  String get reason;

  ChannelReadOnlySet._();

  factory ChannelReadOnlySet([void updates(ChannelReadOnlySetBuilder b)]) =
      _$ChannelReadOnlySet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelReadOnlySetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelReadOnlySet> get serializer =>
      _$ChannelReadOnlySetSerializer();
}

class _$ChannelReadOnlySetSerializer
    implements PrimitiveSerializer<ChannelReadOnlySet> {
  @override
  final Iterable<Type> types = const [ChannelReadOnlySet, _$ChannelReadOnlySet];

  @override
  final String wireName = r'ChannelReadOnlySet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelReadOnlySet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'read_only';
    yield serializers.serialize(
      object.readOnly,
      specifiedType: const FullType(bool),
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
    ChannelReadOnlySet object, {
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
    required ChannelReadOnlySetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'read_only':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.readOnly = valueDes;
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
  ChannelReadOnlySet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelReadOnlySetBuilder();
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
