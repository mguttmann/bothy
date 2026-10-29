//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_folder.g.dart';

/// ProjectFolder
///
/// Properties:
/// * [name] - One folder name directly under a runner's projects root, the one-segment form of `workspace_ref` (09 18.12 item 2).
@BuiltValue()
abstract class ProjectFolder
    implements Built<ProjectFolder, ProjectFolderBuilder> {
  /// One folder name directly under a runner's projects root, the one-segment form of `workspace_ref` (09 18.12 item 2).
  @BuiltValueField(wireName: r'name')
  String get name;

  ProjectFolder._();

  factory ProjectFolder([void updates(ProjectFolderBuilder b)]) =
      _$ProjectFolder;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectFolderBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectFolder> get serializer =>
      _$ProjectFolderSerializer();
}

class _$ProjectFolderSerializer implements PrimitiveSerializer<ProjectFolder> {
  @override
  final Iterable<Type> types = const [ProjectFolder, _$ProjectFolder];

  @override
  final String wireName = r'ProjectFolder';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectFolder object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectFolder object, {
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
    required ProjectFolderBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectFolder deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectFolderBuilder();
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
