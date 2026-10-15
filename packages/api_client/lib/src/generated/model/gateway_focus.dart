//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_focus.g.dart';

/// GatewayFocus
///
/// Properties:
/// * [threadIds] - Complete set of focused threads; replaces the previous set. More than `sync.max_focused_threads` ids answer `error` with `validation_failed` and keep the previous set.
/// * [idle] - True when the window is open but unused; live-box patches continue, but the person does not count as watching (13, 4.4).
@BuiltValue()
abstract class GatewayFocus
    implements Built<GatewayFocus, GatewayFocusBuilder> {
  /// Complete set of focused threads; replaces the previous set. More than `sync.max_focused_threads` ids answer `error` with `validation_failed` and keep the previous set.
  @BuiltValueField(wireName: r'thread_ids')
  BuiltList<String> get threadIds;

  /// True when the window is open but unused; live-box patches continue, but the person does not count as watching (13, 4.4).
  @BuiltValueField(wireName: r'idle')
  bool? get idle;

  GatewayFocus._();

  factory GatewayFocus([void updates(GatewayFocusBuilder b)]) = _$GatewayFocus;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayFocusBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayFocus> get serializer => _$GatewayFocusSerializer();
}

class _$GatewayFocusSerializer implements PrimitiveSerializer<GatewayFocus> {
  @override
  final Iterable<Type> types = const [GatewayFocus, _$GatewayFocus];

  @override
  final String wireName = r'GatewayFocus';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayFocus object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_ids';
    yield serializers.serialize(
      object.threadIds,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    if (object.idle != null) {
      yield r'idle';
      yield serializers.serialize(
        object.idle,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayFocus object, {
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
    required GatewayFocusBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.threadIds.replace(valueDes);
          break;
        case r'idle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.idle = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayFocus deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayFocusBuilder();
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
