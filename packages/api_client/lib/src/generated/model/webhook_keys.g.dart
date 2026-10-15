// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'webhook_keys.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$WebhookKeys extends WebhookKeys {
  @override
  final BuiltList<WebhookKey> keys;

  factory _$WebhookKeys([void Function(WebhookKeysBuilder)? updates]) =>
      (WebhookKeysBuilder()..update(updates))._build();

  _$WebhookKeys._({required this.keys}) : super._();
  @override
  WebhookKeys rebuild(void Function(WebhookKeysBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  WebhookKeysBuilder toBuilder() => WebhookKeysBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is WebhookKeys && keys == other.keys;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, keys.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'WebhookKeys',
    )..add('keys', keys)).toString();
  }
}

class WebhookKeysBuilder implements Builder<WebhookKeys, WebhookKeysBuilder> {
  _$WebhookKeys? _$v;

  ListBuilder<WebhookKey>? _keys;
  ListBuilder<WebhookKey> get keys =>
      _$this._keys ??= ListBuilder<WebhookKey>();
  set keys(ListBuilder<WebhookKey>? keys) => _$this._keys = keys;

  WebhookKeysBuilder() {
    WebhookKeys._defaults(this);
  }

  WebhookKeysBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _keys = $v.keys.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(WebhookKeys other) {
    _$v = other as _$WebhookKeys;
  }

  @override
  void update(void Function(WebhookKeysBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  WebhookKeys build() => _build();

  _$WebhookKeys _build() {
    _$WebhookKeys _$result;
    try {
      _$result = _$v ?? _$WebhookKeys._(keys: keys.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'keys';
        keys.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'WebhookKeys',
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
