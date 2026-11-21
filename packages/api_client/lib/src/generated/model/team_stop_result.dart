//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'team_stop_result.g.dart';

/// TeamStopResult
///
/// Properties:
/// * [stoppedRunIds] - The runs the stop ended or asked to end.
@BuiltValue()
abstract class TeamStopResult
    implements Built<TeamStopResult, TeamStopResultBuilder> {
  /// The runs the stop ended or asked to end.
  @BuiltValueField(wireName: r'stopped_run_ids')
  BuiltList<String> get stoppedRunIds;

  TeamStopResult._();

  factory TeamStopResult([void updates(TeamStopResultBuilder b)]) =
      _$TeamStopResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TeamStopResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TeamStopResult> get serializer =>
      _$TeamStopResultSerializer();
}

class _$TeamStopResultSerializer
    implements PrimitiveSerializer<TeamStopResult> {
  @override
  final Iterable<Type> types = const [TeamStopResult, _$TeamStopResult];

  @override
  final String wireName = r'TeamStopResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TeamStopResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'stopped_run_ids';
    yield serializers.serialize(
      object.stoppedRunIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TeamStopResult object, {
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
    required TeamStopResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'stopped_run_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.stoppedRunIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TeamStopResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TeamStopResultBuilder();
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
