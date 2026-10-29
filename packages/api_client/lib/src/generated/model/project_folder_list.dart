//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/project_folder.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_folder_list.g.dart';

/// ProjectFolderList
///
/// Properties:
/// * [items] - The reported folders without a binding, in name order.
/// * [reported] - Whether the runner has reported its folders at least once; false for a runner that never did (an older runner or not yet connected).
@BuiltValue()
abstract class ProjectFolderList
    implements Built<ProjectFolderList, ProjectFolderListBuilder> {
  /// The reported folders without a binding, in name order.
  @BuiltValueField(wireName: r'items')
  BuiltList<ProjectFolder> get items;

  /// Whether the runner has reported its folders at least once; false for a runner that never did (an older runner or not yet connected).
  @BuiltValueField(wireName: r'reported')
  bool get reported;

  ProjectFolderList._();

  factory ProjectFolderList([void updates(ProjectFolderListBuilder b)]) =
      _$ProjectFolderList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectFolderListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectFolderList> get serializer =>
      _$ProjectFolderListSerializer();
}

class _$ProjectFolderListSerializer
    implements PrimitiveSerializer<ProjectFolderList> {
  @override
  final Iterable<Type> types = const [ProjectFolderList, _$ProjectFolderList];

  @override
  final String wireName = r'ProjectFolderList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectFolderList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ProjectFolder)]),
    );
    yield r'reported';
    yield serializers.serialize(
      object.reported,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectFolderList object, {
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
    required ProjectFolderListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(ProjectFolder)]),
          ) as BuiltList<ProjectFolder>;
          result.items.replace(valueDes);
          break;
        case r'reported':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.reported = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectFolderList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectFolderListBuilder();
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
