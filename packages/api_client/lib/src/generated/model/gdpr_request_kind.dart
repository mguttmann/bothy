//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gdpr_request_kind.g.dart';

class GdprRequestKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'export')
  static const GdprRequestKind export_ = _$export_;
  @BuiltValueEnumConst(wireName: r'deletion')
  static const GdprRequestKind deletion = _$deletion;
  @BuiltValueEnumConst(wireName: r'rectification')
  static const GdprRequestKind rectification = _$rectification;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const GdprRequestKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<GdprRequestKind> get serializer =>
      _$gdprRequestKindSerializer;

  const GdprRequestKind._(String name) : super(name);

  static BuiltSet<GdprRequestKind> get values => _$values;
  static GdprRequestKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class GdprRequestKindMixin = Object with _$GdprRequestKindMixin;
