//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/host_computer_app.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_state.g.dart';

/// HostComputerState
///
/// Properties:
/// * [sessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [reason] - `locked` exactly with `paused`, empty otherwise.
/// * [actionCount]
/// * [frontApp]
@BuiltValue()
abstract class HostComputerState
    implements Built<HostComputerState, HostComputerStateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'session_id')
  String get sessionId;

  @BuiltValueField(wireName: r'state')
  HostComputerStateStateEnum get state;
  // enum stateEnum {  active,  paused,  };

  /// `locked` exactly with `paused`, empty otherwise.
  @BuiltValueField(wireName: r'reason')
  HostComputerStateReasonEnum get reason;
  // enum reasonEnum {  ,  locked,  };

  @BuiltValueField(wireName: r'action_count')
  int get actionCount;

  @BuiltValueField(wireName: r'front_app')
  HostComputerApp get frontApp;

  HostComputerState._();

  factory HostComputerState([void updates(HostComputerStateBuilder b)]) =
      _$HostComputerState;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerStateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerState> get serializer =>
      _$HostComputerStateSerializer();
}

class _$HostComputerStateSerializer
    implements PrimitiveSerializer<HostComputerState> {
  @override
  final Iterable<Type> types = const [HostComputerState, _$HostComputerState];

  @override
  final String wireName = r'HostComputerState';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerState object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'session_id';
    yield serializers.serialize(
      object.sessionId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HostComputerStateStateEnum),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(HostComputerStateReasonEnum),
    );
    yield r'action_count';
    yield serializers.serialize(
      object.actionCount,
      specifiedType: const FullType(int),
    );
    yield r'front_app';
    yield serializers.serialize(
      object.frontApp,
      specifiedType: const FullType(HostComputerApp),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerState object, {
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
    required HostComputerStateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.sessionId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerStateStateEnum),
          ) as HostComputerStateStateEnum;
          result.state = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerStateReasonEnum),
          ) as HostComputerStateReasonEnum;
          result.reason = valueDes;
          break;
        case r'action_count':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.actionCount = valueDes;
          break;
        case r'front_app':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerApp),
          ) as HostComputerApp;
          result.frontApp.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerStateBuilder();
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

class HostComputerStateStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'active')
  static const HostComputerStateStateEnum active =
      _$hostComputerStateStateEnum_active;
  @BuiltValueEnumConst(wireName: r'paused')
  static const HostComputerStateStateEnum paused =
      _$hostComputerStateStateEnum_paused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerStateStateEnum unknownDefaultOpenApi =
      _$hostComputerStateStateEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerStateStateEnum> get serializer =>
      _$hostComputerStateStateEnumSerializer;

  const HostComputerStateStateEnum._(String name) : super(name);

  static BuiltSet<HostComputerStateStateEnum> get values =>
      _$hostComputerStateStateEnumValues;
  static HostComputerStateStateEnum valueOf(String name) =>
      _$hostComputerStateStateEnumValueOf(name);
}

/// `locked` exactly with `paused`, empty otherwise.
class HostComputerStateReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'')
  static const HostComputerStateReasonEnum empty =
      _$hostComputerStateReasonEnum_empty;
  @BuiltValueEnumConst(wireName: r'locked')
  static const HostComputerStateReasonEnum locked =
      _$hostComputerStateReasonEnum_locked;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerStateReasonEnum unknownDefaultOpenApi =
      _$hostComputerStateReasonEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerStateReasonEnum> get serializer =>
      _$hostComputerStateReasonEnumSerializer;

  const HostComputerStateReasonEnum._(String name) : super(name);

  static BuiltSet<HostComputerStateReasonEnum> get values =>
      _$hostComputerStateReasonEnumValues;
  static HostComputerStateReasonEnum valueOf(String name) =>
      _$hostComputerStateReasonEnumValueOf(name);
}
