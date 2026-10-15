// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_preview.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PushPreview extends PushPreview {
  @override
  final NotificationKind kind;
  @override
  final String? threadId;
  @override
  final String? messageId;
  @override
  final int badge;
  @override
  final String? spaceName;
  @override
  final String? channelName;
  @override
  final String? threadTitle;
  @override
  final String? senderDisplayName;
  @override
  final String? excerpt;
  @override
  final int? attachmentCount;
  @override
  final String? imageUrl;

  factory _$PushPreview([void Function(PushPreviewBuilder)? updates]) =>
      (PushPreviewBuilder()..update(updates))._build();

  _$PushPreview._({
    required this.kind,
    this.threadId,
    this.messageId,
    required this.badge,
    this.spaceName,
    this.channelName,
    this.threadTitle,
    this.senderDisplayName,
    this.excerpt,
    this.attachmentCount,
    this.imageUrl,
  }) : super._();
  @override
  PushPreview rebuild(void Function(PushPreviewBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PushPreviewBuilder toBuilder() => PushPreviewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PushPreview &&
        kind == other.kind &&
        threadId == other.threadId &&
        messageId == other.messageId &&
        badge == other.badge &&
        spaceName == other.spaceName &&
        channelName == other.channelName &&
        threadTitle == other.threadTitle &&
        senderDisplayName == other.senderDisplayName &&
        excerpt == other.excerpt &&
        attachmentCount == other.attachmentCount &&
        imageUrl == other.imageUrl;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, badge.hashCode);
    _$hash = $jc(_$hash, spaceName.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jc(_$hash, threadTitle.hashCode);
    _$hash = $jc(_$hash, senderDisplayName.hashCode);
    _$hash = $jc(_$hash, excerpt.hashCode);
    _$hash = $jc(_$hash, attachmentCount.hashCode);
    _$hash = $jc(_$hash, imageUrl.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'PushPreview')
          ..add('kind', kind)
          ..add('threadId', threadId)
          ..add('messageId', messageId)
          ..add('badge', badge)
          ..add('spaceName', spaceName)
          ..add('channelName', channelName)
          ..add('threadTitle', threadTitle)
          ..add('senderDisplayName', senderDisplayName)
          ..add('excerpt', excerpt)
          ..add('attachmentCount', attachmentCount)
          ..add('imageUrl', imageUrl))
        .toString();
  }
}

class PushPreviewBuilder implements Builder<PushPreview, PushPreviewBuilder> {
  _$PushPreview? _$v;

  NotificationKind? _kind;
  NotificationKind? get kind => _$this._kind;
  set kind(NotificationKind? kind) => _$this._kind = kind;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  int? _badge;
  int? get badge => _$this._badge;
  set badge(int? badge) => _$this._badge = badge;

  String? _spaceName;
  String? get spaceName => _$this._spaceName;
  set spaceName(String? spaceName) => _$this._spaceName = spaceName;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  String? _threadTitle;
  String? get threadTitle => _$this._threadTitle;
  set threadTitle(String? threadTitle) => _$this._threadTitle = threadTitle;

  String? _senderDisplayName;
  String? get senderDisplayName => _$this._senderDisplayName;
  set senderDisplayName(String? senderDisplayName) =>
      _$this._senderDisplayName = senderDisplayName;

  String? _excerpt;
  String? get excerpt => _$this._excerpt;
  set excerpt(String? excerpt) => _$this._excerpt = excerpt;

  int? _attachmentCount;
  int? get attachmentCount => _$this._attachmentCount;
  set attachmentCount(int? attachmentCount) =>
      _$this._attachmentCount = attachmentCount;

  String? _imageUrl;
  String? get imageUrl => _$this._imageUrl;
  set imageUrl(String? imageUrl) => _$this._imageUrl = imageUrl;

  PushPreviewBuilder() {
    PushPreview._defaults(this);
  }

  PushPreviewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _threadId = $v.threadId;
      _messageId = $v.messageId;
      _badge = $v.badge;
      _spaceName = $v.spaceName;
      _channelName = $v.channelName;
      _threadTitle = $v.threadTitle;
      _senderDisplayName = $v.senderDisplayName;
      _excerpt = $v.excerpt;
      _attachmentCount = $v.attachmentCount;
      _imageUrl = $v.imageUrl;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PushPreview other) {
    _$v = other as _$PushPreview;
  }

  @override
  void update(void Function(PushPreviewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PushPreview build() => _build();

  _$PushPreview _build() {
    final _$result =
        _$v ??
        _$PushPreview._(
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'PushPreview',
            'kind',
          ),
          threadId: threadId,
          messageId: messageId,
          badge: BuiltValueNullFieldError.checkNotNull(
            badge,
            r'PushPreview',
            'badge',
          ),
          spaceName: spaceName,
          channelName: channelName,
          threadTitle: threadTitle,
          senderDisplayName: senderDisplayName,
          excerpt: excerpt,
          attachmentCount: attachmentCount,
          imageUrl: imageUrl,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
