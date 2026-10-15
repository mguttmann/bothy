// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction_respond.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const InteractionRespondStateEnum _$interactionRespondStateEnum_acknowledged =
    const InteractionRespondStateEnum._('acknowledged');
const InteractionRespondStateEnum _$interactionRespondStateEnum_failed =
    const InteractionRespondStateEnum._('failed');
const InteractionRespondStateEnum _$interactionRespondStateEnum_rejected =
    const InteractionRespondStateEnum._('rejected');
const InteractionRespondStateEnum
_$interactionRespondStateEnum_unknownDefaultOpenApi =
    const InteractionRespondStateEnum._('unknownDefaultOpenApi');

InteractionRespondStateEnum _$interactionRespondStateEnumValueOf(String name) {
  switch (name) {
    case 'acknowledged':
      return _$interactionRespondStateEnum_acknowledged;
    case 'failed':
      return _$interactionRespondStateEnum_failed;
    case 'rejected':
      return _$interactionRespondStateEnum_rejected;
    case 'unknownDefaultOpenApi':
      return _$interactionRespondStateEnum_unknownDefaultOpenApi;
    default:
      return _$interactionRespondStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<InteractionRespondStateEnum>
_$interactionRespondStateEnumValues = BuiltSet<InteractionRespondStateEnum>(
  const <InteractionRespondStateEnum>[
    _$interactionRespondStateEnum_acknowledged,
    _$interactionRespondStateEnum_failed,
    _$interactionRespondStateEnum_rejected,
    _$interactionRespondStateEnum_unknownDefaultOpenApi,
  ],
);

Serializer<InteractionRespondStateEnum>
_$interactionRespondStateEnumSerializer =
    _$InteractionRespondStateEnumSerializer();

class _$InteractionRespondStateEnumSerializer
    implements PrimitiveSerializer<InteractionRespondStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'acknowledged': 'acknowledged',
    'failed': 'failed',
    'rejected': 'rejected',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'acknowledged': 'acknowledged',
    'failed': 'failed',
    'rejected': 'rejected',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[InteractionRespondStateEnum];
  @override
  final String wireName = 'InteractionRespondStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    InteractionRespondStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  InteractionRespondStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => InteractionRespondStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$InteractionRespond extends InteractionRespond {
  @override
  final InteractionRespondStateEnum state;
  @override
  final InteractionResponse? response;

  factory _$InteractionRespond([
    void Function(InteractionRespondBuilder)? updates,
  ]) => (InteractionRespondBuilder()..update(updates))._build();

  _$InteractionRespond._({required this.state, this.response}) : super._();
  @override
  InteractionRespond rebuild(
    void Function(InteractionRespondBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InteractionRespondBuilder toBuilder() =>
      InteractionRespondBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InteractionRespond &&
        state == other.state &&
        response == other.response;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, response.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InteractionRespond')
          ..add('state', state)
          ..add('response', response))
        .toString();
  }
}

class InteractionRespondBuilder
    implements Builder<InteractionRespond, InteractionRespondBuilder> {
  _$InteractionRespond? _$v;

  InteractionRespondStateEnum? _state;
  InteractionRespondStateEnum? get state => _$this._state;
  set state(InteractionRespondStateEnum? state) => _$this._state = state;

  InteractionResponseBuilder? _response;
  InteractionResponseBuilder get response =>
      _$this._response ??= InteractionResponseBuilder();
  set response(InteractionResponseBuilder? response) =>
      _$this._response = response;

  InteractionRespondBuilder() {
    InteractionRespond._defaults(this);
  }

  InteractionRespondBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _response = $v.response?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InteractionRespond other) {
    _$v = other as _$InteractionRespond;
  }

  @override
  void update(void Function(InteractionRespondBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InteractionRespond build() => _build();

  _$InteractionRespond _build() {
    _$InteractionRespond _$result;
    try {
      _$result =
          _$v ??
          _$InteractionRespond._(
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'InteractionRespond',
              'state',
            ),
            response: _response?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'response';
        _response?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InteractionRespond',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
