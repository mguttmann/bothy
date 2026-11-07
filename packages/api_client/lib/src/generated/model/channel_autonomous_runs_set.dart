//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_autonomous_runs_set.g.dart';

/// ChannelAutonomousRunsSet
///
/// Properties:
/// * [allowAutonomousRuns]
@BuiltValue()
abstract class ChannelAutonomousRunsSet
    implements
        Built<ChannelAutonomousRunsSet, ChannelAutonomousRunsSetBuilder> {
  @BuiltValueField(wireName: r'allow_autonomous_runs')
  bool get allowAutonomousRuns;

  ChannelAutonomousRunsSet._();

  factory ChannelAutonomousRunsSet([
    void updates(ChannelAutonomousRunsSetBuilder b),
  ]) = _$ChannelAutonomousRunsSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelAutonomousRunsSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelAutonomousRunsSet> get serializer =>
      _$ChannelAutonomousRunsSetSerializer();
}

class _$ChannelAutonomousRunsSetSerializer
    implements PrimitiveSerializer<ChannelAutonomousRunsSet> {
  @override
  final Iterable<Type> types = const [
    ChannelAutonomousRunsSet,
    _$ChannelAutonomousRunsSet,
  ];

  @override
  final String wireName = r'ChannelAutonomousRunsSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelAutonomousRunsSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'allow_autonomous_runs';
    yield serializers.serialize(
      object.allowAutonomousRuns,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelAutonomousRunsSet object, {
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
    required ChannelAutonomousRunsSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'allow_autonomous_runs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.allowAutonomousRuns = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelAutonomousRunsSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelAutonomousRunsSetBuilder();
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
