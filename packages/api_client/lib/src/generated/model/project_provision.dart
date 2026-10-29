//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'project_provision.g.dart';

/// Whether the serving runner creates the folder of the binding (09 18.12). `none` creates nothing and the folder must exist; `init` creates an empty folder with `git init`; `clone` clones `repository_url`, which must be `https`. With `init` or `clone` the `workspace_ref` is one segment of `^[A-Za-z0-9][A-Za-z0-9._-]*$`. Absent means `none`; the server always sends it.
class ProjectProvision extends EnumClass {
  @BuiltValueEnumConst(wireName: r'none')
  static const ProjectProvision none = _$none;
  @BuiltValueEnumConst(wireName: r'init')
  static const ProjectProvision init = _$init;
  @BuiltValueEnumConst(wireName: r'clone')
  static const ProjectProvision clone = _$clone;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ProjectProvision unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ProjectProvision> get serializer =>
      _$projectProvisionSerializer;

  const ProjectProvision._(String name) : super(name);

  static BuiltSet<ProjectProvision> get values => _$values;
  static ProjectProvision valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ProjectProvisionMixin = Object with _$ProjectProvisionMixin;
