//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_trash_request.g.dart';

/// A project folder the runner moves to the trash (09 18.15).
///
/// Properties:
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [workspaceRef]
@BuiltValue()
abstract class ProjectTrashRequest
    implements Built<ProjectTrashRequest, ProjectTrashRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  @BuiltValueField(wireName: r'workspace_ref')
  String get workspaceRef;

  ProjectTrashRequest._();

  factory ProjectTrashRequest([void updates(ProjectTrashRequestBuilder b)]) =
      _$ProjectTrashRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectTrashRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectTrashRequest> get serializer =>
      _$ProjectTrashRequestSerializer();
}

class _$ProjectTrashRequestSerializer
    implements PrimitiveSerializer<ProjectTrashRequest> {
  @override
  final Iterable<Type> types = const [
    ProjectTrashRequest,
    _$ProjectTrashRequest,
  ];

  @override
  final String wireName = r'ProjectTrashRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectTrashRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    yield r'workspace_ref';
    yield serializers.serialize(
      object.workspaceRef,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectTrashRequest object, {
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
    required ProjectTrashRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'workspace_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.workspaceRef = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectTrashRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectTrashRequestBuilder();
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
