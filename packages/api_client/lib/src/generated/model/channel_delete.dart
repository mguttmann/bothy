//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_delete.g.dart';

/// ChannelDelete
///
/// Properties:
/// * [trashProjectFolder] - Also move the project folder of a forum with a project binding to the trash of the serving runner's host once the deletion is final (09 18.15). Needs `channel.project.bind` and an ok folder check.
@BuiltValue()
abstract class ChannelDelete
    implements Built<ChannelDelete, ChannelDeleteBuilder> {
  /// Also move the project folder of a forum with a project binding to the trash of the serving runner's host once the deletion is final (09 18.15). Needs `channel.project.bind` and an ok folder check.
  @BuiltValueField(wireName: r'trash_project_folder')
  bool? get trashProjectFolder;

  ChannelDelete._();

  factory ChannelDelete([void updates(ChannelDeleteBuilder b)]) =
      _$ChannelDelete;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ChannelDeleteBuilder b) =>
      b..trashProjectFolder = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<ChannelDelete> get serializer =>
      _$ChannelDeleteSerializer();
}

class _$ChannelDeleteSerializer implements PrimitiveSerializer<ChannelDelete> {
  @override
  final Iterable<Type> types = const [ChannelDelete, _$ChannelDelete];

  @override
  final String wireName = r'ChannelDelete';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ChannelDelete object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.trashProjectFolder != null) {
      yield r'trash_project_folder';
      yield serializers.serialize(
        object.trashProjectFolder,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ChannelDelete object, {
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
    required ChannelDeleteBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'trash_project_folder':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.trashProjectFolder = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ChannelDelete deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ChannelDeleteBuilder();
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
