//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_wake.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_wake_page.g.dart';

/// One page. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class RunWakePage implements Built<RunWakePage, RunWakePageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<RunWake> get items;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  RunWakePage._();

  factory RunWakePage([void updates(RunWakePageBuilder b)]) = _$RunWakePage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunWakePageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunWakePage> get serializer => _$RunWakePageSerializer();
}

class _$RunWakePageSerializer implements PrimitiveSerializer<RunWakePage> {
  @override
  final Iterable<Type> types = const [RunWakePage, _$RunWakePage];

  @override
  final String wireName = r'RunWakePage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunWakePage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(RunWake)]),
    );
    if (object.nextCursor != null) {
      yield r'next_cursor';
      yield serializers.serialize(
        object.nextCursor,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunWakePage object, {
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
    required RunWakePageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(RunWake)]),
          ) as BuiltList<RunWake>;
          result.items.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.nextCursor = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunWakePage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunWakePageBuilder();
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
