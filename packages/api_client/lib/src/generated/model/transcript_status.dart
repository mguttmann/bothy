//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'transcript_status.g.dart';

class TranscriptStatus extends EnumClass {
  @BuiltValueEnumConst(wireName: r'none')
  static const TranscriptStatus none = _$none;
  @BuiltValueEnumConst(wireName: r'pending')
  static const TranscriptStatus pending = _$pending;
  @BuiltValueEnumConst(wireName: r'done')
  static const TranscriptStatus done = _$done;
  @BuiltValueEnumConst(wireName: r'failed')
  static const TranscriptStatus failed = _$failed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TranscriptStatus unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<TranscriptStatus> get serializer =>
      _$transcriptStatusSerializer;

  const TranscriptStatus._(String name) : super(name);

  static BuiltSet<TranscriptStatus> get values => _$values;
  static TranscriptStatus valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TranscriptStatusMixin = Object with _$TranscriptStatusMixin;
