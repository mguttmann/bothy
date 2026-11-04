//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_history_entry.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_history_page.g.dart';

/// One page of the run history. `next_cursor` is absent on the last page, which ends with the oldest run the caller may read.
///
/// Properties:
/// * [items]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class RunHistoryPage
    implements Built<RunHistoryPage, RunHistoryPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<RunHistoryEntry> get items;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  RunHistoryPage._();

  factory RunHistoryPage([void updates(RunHistoryPageBuilder b)]) =
      _$RunHistoryPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunHistoryPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunHistoryPage> get serializer =>
      _$RunHistoryPageSerializer();
}

class _$RunHistoryPageSerializer
    implements PrimitiveSerializer<RunHistoryPage> {
  @override
  final Iterable<Type> types = const [RunHistoryPage, _$RunHistoryPage];

  @override
  final String wireName = r'RunHistoryPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunHistoryPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(RunHistoryEntry)]),
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
    RunHistoryPage object, {
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
    required RunHistoryPageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(RunHistoryEntry),
            ]),
          ) as BuiltList<RunHistoryEntry>;
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
  RunHistoryPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunHistoryPageBuilder();
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
