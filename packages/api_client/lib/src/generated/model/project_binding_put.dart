//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/project_provision.dart';
import 'package:api_client/src/generated/model/project_runner_config.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_binding_put.g.dart';

/// ProjectBindingPut
///
/// Properties:
/// * [workspaceRef]
/// * [repositoryUrl]
/// * [defaultBranch]
/// * [runnerConfig]
/// * [provision]
/// * [reason]
@BuiltValue()
abstract class ProjectBindingPut
    implements Built<ProjectBindingPut, ProjectBindingPutBuilder> {
  @BuiltValueField(wireName: r'workspace_ref')
  String get workspaceRef;

  @BuiltValueField(wireName: r'repository_url')
  String? get repositoryUrl;

  @BuiltValueField(wireName: r'default_branch')
  String? get defaultBranch;

  @BuiltValueField(wireName: r'runner_config')
  ProjectRunnerConfig? get runnerConfig;

  @BuiltValueField(wireName: r'provision')
  ProjectProvision? get provision;
  // enum provisionEnum {  none,  init,  clone,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

  ProjectBindingPut._();

  factory ProjectBindingPut([void updates(ProjectBindingPutBuilder b)]) =
      _$ProjectBindingPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectBindingPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectBindingPut> get serializer =>
      _$ProjectBindingPutSerializer();
}

class _$ProjectBindingPutSerializer
    implements PrimitiveSerializer<ProjectBindingPut> {
  @override
  final Iterable<Type> types = const [ProjectBindingPut, _$ProjectBindingPut];

  @override
  final String wireName = r'ProjectBindingPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectBindingPut object, {
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
    if (object.runnerConfig != null) {
      yield r'runner_config';
      yield serializers.serialize(
        object.runnerConfig,
        specifiedType: const FullType(ProjectRunnerConfig),
      );
    }
    if (object.provision != null) {
      yield r'provision';
      yield serializers.serialize(
        object.provision,
        specifiedType: const FullType(ProjectProvision),
      );
    }
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectBindingPut object, {
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
    required ProjectBindingPutBuilder result,
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
            specifiedType: const FullType.nullable(ProjectRunnerConfig),
          ) as ProjectRunnerConfig?;
          if (valueDes == null) continue;
          result.runnerConfig.replace(valueDes);
          break;
        case r'provision':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(ProjectProvision),
          ) as ProjectProvision?;
          if (valueDes == null) continue;
          result.provision = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ProjectBindingPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectBindingPutBuilder();
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
