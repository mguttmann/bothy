// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhookCreate extends IncomingWebhookCreate {
  @override
  final String appId;
  @override
  final String name;
  @override
  final String? threadId;

  factory _$IncomingWebhookCreate([
    void Function(IncomingWebhookCreateBuilder)? updates,
  ]) => (IncomingWebhookCreateBuilder()..update(updates))._build();

  _$IncomingWebhookCreate._({
    required this.appId,
    required this.name,
    this.threadId,
  }) : super._();
  @override
  IncomingWebhookCreate rebuild(
    void Function(IncomingWebhookCreateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookCreateBuilder toBuilder() =>
      IncomingWebhookCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookCreate &&
        appId == other.appId &&
        name == other.name &&
        threadId == other.threadId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, appId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhookCreate')
          ..add('appId', appId)
          ..add('name', name)
          ..add('threadId', threadId))
        .toString();
  }
}

class IncomingWebhookCreateBuilder
    implements Builder<IncomingWebhookCreate, IncomingWebhookCreateBuilder> {
  _$IncomingWebhookCreate? _$v;

  String? _appId;
  String? get appId => _$this._appId;
  set appId(String? appId) => _$this._appId = appId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  IncomingWebhookCreateBuilder() {
    IncomingWebhookCreate._defaults(this);
  }

  IncomingWebhookCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _appId = $v.appId;
      _name = $v.name;
      _threadId = $v.threadId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookCreate other) {
    _$v = other as _$IncomingWebhookCreate;
  }

  @override
  void update(void Function(IncomingWebhookCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookCreate build() => _build();

  _$IncomingWebhookCreate _build() {
    final _$result =
        _$v ??
        _$IncomingWebhookCreate._(
          appId: BuiltValueNullFieldError.checkNotNull(
            appId,
            r'IncomingWebhookCreate',
            'appId',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'IncomingWebhookCreate',
            'name',
          ),
          threadId: threadId,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
