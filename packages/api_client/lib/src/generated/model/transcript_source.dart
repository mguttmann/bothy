//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'transcript_source.g.dart';

class TranscriptSource extends EnumClass {
  @BuiltValueEnumConst(wireName: r'agent')
  static const TranscriptSource agent = _$agent;
  @BuiltValueEnumConst(wireName: r'service')
  static const TranscriptSource service = _$service;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TranscriptSource unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<TranscriptSource> get serializer =>
      _$transcriptSourceSerializer;

  const TranscriptSource._(String name) : super(name);

  static BuiltSet<TranscriptSource> get values => _$values;
  static TranscriptSource valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class TranscriptSourceMixin = Object with _$TranscriptSourceMixin;
