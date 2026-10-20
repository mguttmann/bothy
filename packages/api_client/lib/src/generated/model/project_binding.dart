//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/project_runner_config.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_binding.g.dart';

/// ProjectBinding
///
/// Properties:
/// * [workspaceRef] - Opaque reference the runner resolves; never a server path.
/// * [repositoryUrl] - Display only; the server never fetches this URL.
/// * [defaultBranch]
/// * [runnerConfig]
/// * [ready]
/// * [version]
@BuiltValue()
abstract class ProjectBinding
    implements Built<ProjectBinding, ProjectBindingBuilder> {
  /// Opaque reference the runner resolves; never a server path.
  @BuiltValueField(wireName: r'workspace_ref')
  String get workspaceRef;

  /// Display only; the server never fetches this URL.
  @BuiltValueField(wireName: r'repository_url')
  String? get repositoryUrl;

  @BuiltValueField(wireName: r'default_branch')
  String? get defaultBranch;

  @BuiltValueField(wireName: r'runner_config')
  ProjectRunnerConfig get runnerConfig;

  @BuiltValueField(wireName: r'ready')
  bool get ready;

  @BuiltValueField(wireName: r'version')
  int get version;

  ProjectBinding._();

  factory ProjectBinding([void updates(ProjectBindingBuilder b)]) =
      _$ProjectBinding;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectBindingBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectBinding> get serializer =>
      _$ProjectBindingSerializer();
}

class _$ProjectBindingSerializer
    implements PrimitiveSerializer<ProjectBinding> {
  @override
  final Iterable<Type> types = const [ProjectBinding, _$ProjectBinding];

  @override
  final String wireName = r'ProjectBinding';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectBinding object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'workspace_ref';
    yield serializers.serialize(
      object.workspaceRef,
      specifiedType: const FullType(String),
    );
    if (object.repositoryUrl != null) {
      yield r'repository_url';
      yield serializers.serialize(
        object.repositoryUrl,
        specifiedType: const FullType(String),
      );
    }
    if (object.defaultBranch != null) {
      yield r'default_branch';
      yield serializers.serialize(
        object.defaultBranch,
        specifiedType: const FullType(String),
      );
    }
    yield r'runner_config';
    yield serializers.serialize(
      object.runnerConfig,
      specifiedType: const FullType(ProjectRunnerConfig),
    );
    yield r'ready';
    yield serializers.serialize(
      object.ready,
      specifiedType: const FullType(bool),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectBinding object, {
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
    required ProjectBindingBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'workspace_ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.workspaceRef = valueDes;
          break;
        case r'repository_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.repositoryUrl = valueDes;
          break;
        case r'default_branch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.defaultBranch = valueDes;
          break;
        case r'runner_config':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ProjectRunnerConfig),
          ) as ProjectRunnerConfig;
          result.runnerConfig.replace(valueDes);
          break;
        case r'ready':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ready = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectBinding deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectBindingBuilder();
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
