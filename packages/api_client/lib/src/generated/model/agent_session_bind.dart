//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_session_bind.g.dart';

/// AgentSessionBind
///
/// Properties:
/// * [externalSessionRef]
/// * [expectedCurrentSessionId] - Current session of the thread as the client saw it; absent when the thread has none. A mismatch answers 409 `conflict` with `reason` `session_changed` (09, 9.4).
/// * [takeOver] - Required when the session is current in another thread; otherwise 409 `conflict` with `reason` `session_bound_elsewhere`.
@BuiltValue()
abstract class AgentSessionBind
    implements Built<AgentSessionBind, AgentSessionBindBuilder> {
  @BuiltValueField(wireName: r'external_session_ref')
  String get externalSessionRef;

  /// Current session of the thread as the client saw it; absent when the thread has none. A mismatch answers 409 `conflict` with `reason` `session_changed` (09, 9.4).
  @BuiltValueField(wireName: r'expected_current_session_id')
  String? get expectedCurrentSessionId;

  /// Required when the session is current in another thread; otherwise 409 `conflict` with `reason` `session_bound_elsewhere`.
  @BuiltValueField(wireName: r'take_over')
  bool? get takeOver;

  AgentSessionBind._();

  factory AgentSessionBind([void updates(AgentSessionBindBuilder b)]) =
      _$AgentSessionBind;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentSessionBindBuilder b) => b..takeOver = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentSessionBind> get serializer =>
      _$AgentSessionBindSerializer();
}

class _$AgentSessionBindSerializer
    implements PrimitiveSerializer<AgentSessionBind> {
  @override
  final Iterable<Type> types = const [AgentSessionBind, _$AgentSessionBind];

  @override
  final String wireName = r'AgentSessionBind';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentSessionBind object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'external_session_ref';
    yield serializers.serialize(
      object.externalSessionRef,
      specifiedType: const FullType(String),
    );
    if (object.expectedCurrentSessionId != null) {
      yield r'expected_current_session_id';
      yield serializers.serialize(
        object.expectedCurrentSessionId,
        specifiedType: const FullType(String),
      );
    }
    if (object.takeOver != null) {
      yield r'take_over';
      yield serializers.serialize(
        object.takeOver,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionBind object, {
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
    required AgentSessionBindBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'external_session_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.externalSessionRef = valueDes;
          break;
        case r'expected_current_session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.expectedCurrentSessionId = valueDes;
          break;
        case r'take_over':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.takeOver = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentSessionBind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentSessionBindBuilder();
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
