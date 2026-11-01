//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_trash_check_report.g.dart';

/// The folder check a runner reports in its readiness entry (09 18.15).
///
/// Properties:
/// * [ok]
/// * [reason] - Why not, with `ok` false; a code of 09 18.15.
@BuiltValue()
abstract class ProjectTrashCheckReport
    implements Built<ProjectTrashCheckReport, ProjectTrashCheckReportBuilder> {
  @BuiltValueField(wireName: r'ok')
  bool get ok;

  /// Why not, with `ok` false; a code of 09 18.15.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  ProjectTrashCheckReport._();

  factory ProjectTrashCheckReport([
    void updates(ProjectTrashCheckReportBuilder b),
  ]) = _$ProjectTrashCheckReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ProjectTrashCheckReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ProjectTrashCheckReport> get serializer =>
      _$ProjectTrashCheckReportSerializer();
}

class _$ProjectTrashCheckReportSerializer
    implements PrimitiveSerializer<ProjectTrashCheckReport> {
  @override
  final Iterable<Type> types = const [
    ProjectTrashCheckReport,
    _$ProjectTrashCheckReport,
  ];

  @override
  final String wireName = r'ProjectTrashCheckReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ProjectTrashCheckReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'ok';
    yield serializers.serialize(object.ok, specifiedType: const FullType(bool));
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ProjectTrashCheckReport object, {
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
    required ProjectTrashCheckReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'ok':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ok = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  ProjectTrashCheckReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ProjectTrashCheckReportBuilder();
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
