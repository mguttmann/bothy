// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'incoming_webhook_post.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$IncomingWebhookPost extends IncomingWebhookPost {
  @override
  final String contentMd;
  @override
  final String? clientMsgId;
  @override
  final String? title;

  factory _$IncomingWebhookPost([
    void Function(IncomingWebhookPostBuilder)? updates,
  ]) => (IncomingWebhookPostBuilder()..update(updates))._build();

  _$IncomingWebhookPost._({
    required this.contentMd,
    this.clientMsgId,
    this.title,
  }) : super._();
  @override
  IncomingWebhookPost rebuild(
    void Function(IncomingWebhookPostBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  IncomingWebhookPostBuilder toBuilder() =>
      IncomingWebhookPostBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is IncomingWebhookPost &&
        contentMd == other.contentMd &&
        clientMsgId == other.clientMsgId &&
        title == other.title;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, contentMd.hashCode);
    _$hash = $jc(_$hash, clientMsgId.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'IncomingWebhookPost')
          ..add('contentMd', contentMd)
          ..add('clientMsgId', clientMsgId)
          ..add('title', title))
        .toString();
  }
}

class IncomingWebhookPostBuilder
    implements Builder<IncomingWebhookPost, IncomingWebhookPostBuilder> {
  _$IncomingWebhookPost? _$v;

  String? _contentMd;
  String? get contentMd => _$this._contentMd;
  set contentMd(String? contentMd) => _$this._contentMd = contentMd;

  String? _clientMsgId;
  String? get clientMsgId => _$this._clientMsgId;
  set clientMsgId(String? clientMsgId) => _$this._clientMsgId = clientMsgId;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  IncomingWebhookPostBuilder() {
    IncomingWebhookPost._defaults(this);
  }

  IncomingWebhookPostBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _contentMd = $v.contentMd;
      _clientMsgId = $v.clientMsgId;
      _title = $v.title;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(IncomingWebhookPost other) {
    _$v = other as _$IncomingWebhookPost;
  }

  @override
  void update(void Function(IncomingWebhookPostBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  IncomingWebhookPost build() => _build();

  _$IncomingWebhookPost _build() {
    final _$result =
        _$v ??
        _$IncomingWebhookPost._(
          contentMd: BuiltValueNullFieldError.checkNotNull(
            contentMd,
            r'IncomingWebhookPost',
            'contentMd',
          ),
          clientMsgId: clientMsgId,
          title: title,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
