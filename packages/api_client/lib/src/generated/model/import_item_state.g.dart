// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_item_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ImportItemState _$pending = const ImportItemState._('pending');
const ImportItemState _$posted = const ImportItemState._('posted');
const ImportItemState _$skipped = const ImportItemState._('skipped');
const ImportItemState _$failed = const ImportItemState._('failed');
const ImportItemState _$unknownDefaultOpenApi = const ImportItemState._(
  'unknownDefaultOpenApi',
);

ImportItemState _$valueOf(String name) {
  switch (name) {
    case 'pending':
      return _$pending;
    case 'posted':
      return _$posted;
    case 'skipped':
      return _$skipped;
    case 'failed':
      return _$failed;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<ImportItemState> _$values = BuiltSet<ImportItemState>(
  const <ImportItemState>[
    _$pending,
    _$posted,
    _$skipped,
    _$failed,
    _$unknownDefaultOpenApi,
  ],
);

class _$ImportItemStateMeta {
  const _$ImportItemStateMeta();
  ImportItemState get pending => _$pending;
  ImportItemState get posted => _$posted;
  ImportItemState get skipped => _$skipped;
  ImportItemState get failed => _$failed;
  ImportItemState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  ImportItemState valueOf(String name) => _$valueOf(name);
  BuiltSet<ImportItemState> get values => _$values;
}

mixin _$ImportItemStateMixin {
  // ignore: non_constant_identifier_names
  _$ImportItemStateMeta get ImportItemState => const _$ImportItemStateMeta();
}

Serializer<ImportItemState> _$importItemStateSerializer =
    _$ImportItemStateSerializer();

class _$ImportItemStateSerializer
    implements PrimitiveSerializer<ImportItemState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'posted': 'posted',
    'skipped': 'skipped',
    'failed': 'failed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'posted': 'posted',
    'skipped': 'skipped',
    'failed': 'failed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ImportItemState];
  @override
  final String wireName = 'ImportItemState';

  @override
  Object serialize(
    Serializers serializers,
    ImportItemState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ImportItemState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ImportItemState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
