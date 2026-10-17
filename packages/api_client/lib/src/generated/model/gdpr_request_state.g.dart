// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GdprRequestState _$requested = const GdprRequestState._('requested');
const GdprRequestState _$approved = const GdprRequestState._('approved');
const GdprRequestState _$running = const GdprRequestState._('running');
const GdprRequestState _$completed = const GdprRequestState._('completed');
const GdprRequestState _$failed = const GdprRequestState._('failed');
const GdprRequestState _$rejected = const GdprRequestState._('rejected');
const GdprRequestState _$cancelled = const GdprRequestState._('cancelled');
const GdprRequestState _$unknownDefaultOpenApi = const GdprRequestState._(
  'unknownDefaultOpenApi',
);

GdprRequestState _$valueOf(String name) {
  switch (name) {
    case 'requested':
      return _$requested;
    case 'approved':
      return _$approved;
    case 'running':
      return _$running;
    case 'completed':
      return _$completed;
    case 'failed':
      return _$failed;
    case 'rejected':
      return _$rejected;
    case 'cancelled':
      return _$cancelled;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<GdprRequestState> _$values = BuiltSet<GdprRequestState>(
  const <GdprRequestState>[
    _$requested,
    _$approved,
    _$running,
    _$completed,
    _$failed,
    _$rejected,
    _$cancelled,
    _$unknownDefaultOpenApi,
  ],
);

class _$GdprRequestStateMeta {
  const _$GdprRequestStateMeta();
  GdprRequestState get requested => _$requested;
  GdprRequestState get approved => _$approved;
  GdprRequestState get running => _$running;
  GdprRequestState get completed => _$completed;
  GdprRequestState get failed => _$failed;
  GdprRequestState get rejected => _$rejected;
  GdprRequestState get cancelled => _$cancelled;
  GdprRequestState get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  GdprRequestState valueOf(String name) => _$valueOf(name);
  BuiltSet<GdprRequestState> get values => _$values;
}

mixin _$GdprRequestStateMixin {
  // ignore: non_constant_identifier_names
  _$GdprRequestStateMeta get GdprRequestState => const _$GdprRequestStateMeta();
}

Serializer<GdprRequestState> _$gdprRequestStateSerializer =
    _$GdprRequestStateSerializer();

class _$GdprRequestStateSerializer
    implements PrimitiveSerializer<GdprRequestState> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'requested': 'requested',
    'approved': 'approved',
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'rejected': 'rejected',
    'cancelled': 'cancelled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'requested': 'requested',
    'approved': 'approved',
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'rejected': 'rejected',
    'cancelled': 'cancelled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GdprRequestState];
  @override
  final String wireName = 'GdprRequestState';

  @override
  Object serialize(
    Serializers serializers,
    GdprRequestState object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GdprRequestState deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GdprRequestState.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
