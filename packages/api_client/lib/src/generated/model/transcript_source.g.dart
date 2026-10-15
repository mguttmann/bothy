// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'transcript_source.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const TranscriptSource _$agent = const TranscriptSource._('agent');
const TranscriptSource _$service = const TranscriptSource._('service');
const TranscriptSource _$unknownDefaultOpenApi = const TranscriptSource._(
  'unknownDefaultOpenApi',
);

TranscriptSource _$valueOf(String name) {
  switch (name) {
    case 'agent':
      return _$agent;
    case 'service':
      return _$service;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<TranscriptSource> _$values = BuiltSet<TranscriptSource>(
  const <TranscriptSource>[_$agent, _$service, _$unknownDefaultOpenApi],
);

class _$TranscriptSourceMeta {
  const _$TranscriptSourceMeta();
  TranscriptSource get agent => _$agent;
  TranscriptSource get service => _$service;
  TranscriptSource get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  TranscriptSource valueOf(String name) => _$valueOf(name);
  BuiltSet<TranscriptSource> get values => _$values;
}

mixin _$TranscriptSourceMixin {
  // ignore: non_constant_identifier_names
  _$TranscriptSourceMeta get TranscriptSource => const _$TranscriptSourceMeta();
}

Serializer<TranscriptSource> _$transcriptSourceSerializer =
    _$TranscriptSourceSerializer();

class _$TranscriptSourceSerializer
    implements PrimitiveSerializer<TranscriptSource> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'agent': 'agent',
    'service': 'service',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'agent': 'agent',
    'service': 'service',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[TranscriptSource];
  @override
  final String wireName = 'TranscriptSource';

  @override
  Object serialize(
    Serializers serializers,
    TranscriptSource object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  TranscriptSource deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => TranscriptSource.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
