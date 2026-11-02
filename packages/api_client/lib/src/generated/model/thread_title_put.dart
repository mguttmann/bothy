//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'thread_title_put.g.dart';

/// ThreadTitlePut
///
/// Properties:
/// * [title]
/// * [auto] - An automatic title (09 section 12 point 5) that never replaces a meaningful title, such as one an agent or a person set on purpose.
@BuiltValue()
abstract class ThreadTitlePut
    implements Built<ThreadTitlePut, ThreadTitlePutBuilder> {
  @BuiltValueField(wireName: r'title')
  String get title;

  /// An automatic title (09 section 12 point 5) that never replaces a meaningful title, such as one an agent or a person set on purpose.
  @BuiltValueField(wireName: r'auto')
  bool? get auto;

  ThreadTitlePut._();

  factory ThreadTitlePut([void updates(ThreadTitlePutBuilder b)]) =
      _$ThreadTitlePut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ThreadTitlePutBuilder b) => b..auto = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<ThreadTitlePut> get serializer =>
      _$ThreadTitlePutSerializer();
}

class _$ThreadTitlePutSerializer
    implements PrimitiveSerializer<ThreadTitlePut> {
  @override
  final Iterable<Type> types = const [ThreadTitlePut, _$ThreadTitlePut];

  @override
  final String wireName = r'ThreadTitlePut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ThreadTitlePut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'title';
    yield serializers.serialize(
      object.title,
      specifiedType: const FullType(String),
    );
    if (object.auto != null) {
      yield r'auto';
      yield serializers.serialize(
        object.auto,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ThreadTitlePut object, {
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
    required ThreadTitlePutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.title = valueDes;
          break;
        case r'auto':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.auto = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ThreadTitlePut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ThreadTitlePutBuilder();
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
