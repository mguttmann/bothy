// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gateway_autocomplete_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GatewayAutocompleteResult extends GatewayAutocompleteResult {
  @override
  final String requestId;
  @override
  final BuiltList<CommandChoice> choices;
  @override
  final String? ref;

  factory _$GatewayAutocompleteResult([
    void Function(GatewayAutocompleteResultBuilder)? updates,
  ]) => (GatewayAutocompleteResultBuilder()..update(updates))._build();

  _$GatewayAutocompleteResult._({
    required this.requestId,
    required this.choices,
    this.ref,
  }) : super._();
  @override
  GatewayAutocompleteResult rebuild(
    void Function(GatewayAutocompleteResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GatewayAutocompleteResultBuilder toBuilder() =>
      GatewayAutocompleteResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GatewayAutocompleteResult &&
        requestId == other.requestId &&
        choices == other.choices &&
        ref == other.ref;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requestId.hashCode);
    _$hash = $jc(_$hash, choices.hashCode);
    _$hash = $jc(_$hash, ref.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GatewayAutocompleteResult')
          ..add('requestId', requestId)
          ..add('choices', choices)
          ..add('ref', ref))
        .toString();
  }
}

class GatewayAutocompleteResultBuilder
    implements
        Builder<GatewayAutocompleteResult, GatewayAutocompleteResultBuilder> {
  _$GatewayAutocompleteResult? _$v;

  String? _requestId;
  String? get requestId => _$this._requestId;
  set requestId(String? requestId) => _$this._requestId = requestId;

  ListBuilder<CommandChoice>? _choices;
  ListBuilder<CommandChoice> get choices =>
      _$this._choices ??= ListBuilder<CommandChoice>();
  set choices(ListBuilder<CommandChoice>? choices) => _$this._choices = choices;

  String? _ref;
  String? get ref => _$this._ref;
  set ref(String? ref) => _$this._ref = ref;

  GatewayAutocompleteResultBuilder() {
    GatewayAutocompleteResult._defaults(this);
  }

  GatewayAutocompleteResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requestId = $v.requestId;
      _choices = $v.choices.toBuilder();
      _ref = $v.ref;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GatewayAutocompleteResult other) {
    _$v = other as _$GatewayAutocompleteResult;
  }

  @override
  void update(void Function(GatewayAutocompleteResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GatewayAutocompleteResult build() => _build();

  _$GatewayAutocompleteResult _build() {
    _$GatewayAutocompleteResult _$result;
    try {
      _$result =
          _$v ??
          _$GatewayAutocompleteResult._(
            requestId: BuiltValueNullFieldError.checkNotNull(
              requestId,
              r'GatewayAutocompleteResult',
              'requestId',
            ),
            choices: choices.build(),
            ref: ref,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'choices';
        choices.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GatewayAutocompleteResult',
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
