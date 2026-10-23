// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_created_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$NotificationCreatedPayload extends NotificationCreatedPayload {
  @override
  final String nid;
  @override
  final NotificationKind kind;
  @override
  final String? threadId;
  @override
  final String? messageId;
  @override
  final String? targetId;
  @override
  final bool quiet;
  @override
  final int count;
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

  factory _$NotificationCreatedPayload([
    void Function(NotificationCreatedPayloadBuilder)? updates,
  ]) => (NotificationCreatedPayloadBuilder()..update(updates))._build();

  _$NotificationCreatedPayload._({
    required this.nid,
    required this.kind,
    this.threadId,
    this.messageId,
    this.targetId,
    required this.quiet,
    required this.count,
    this.spaceName,
    this.channelName,
    this.threadTitle,
    this.senderDisplayName,
    this.excerpt,
    this.attachmentCount,
    this.imageUrl,
  }) : super._();
  @override
  NotificationCreatedPayload rebuild(
    void Function(NotificationCreatedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  NotificationCreatedPayloadBuilder toBuilder() =>
      NotificationCreatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is NotificationCreatedPayload &&
        nid == other.nid &&
        kind == other.kind &&
        threadId == other.threadId &&
        messageId == other.messageId &&
        targetId == other.targetId &&
        quiet == other.quiet &&
        count == other.count &&
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
    _$hash = $jc(_$hash, nid.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, quiet.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
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
    return (newBuiltValueToStringHelper(r'NotificationCreatedPayload')
          ..add('nid', nid)
          ..add('kind', kind)
          ..add('threadId', threadId)
          ..add('messageId', messageId)
          ..add('targetId', targetId)
          ..add('quiet', quiet)
          ..add('count', count)
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

class NotificationCreatedPayloadBuilder
    implements
        Builder<NotificationCreatedPayload, NotificationCreatedPayloadBuilder> {
  _$NotificationCreatedPayload? _$v;

  String? _nid;
  String? get nid => _$this._nid;
  set nid(String? nid) => _$this._nid = nid;

  NotificationKind? _kind;
  NotificationKind? get kind => _$this._kind;
  set kind(NotificationKind? kind) => _$this._kind = kind;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  bool? _quiet;
  bool? get quiet => _$this._quiet;
  set quiet(bool? quiet) => _$this._quiet = quiet;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

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

  NotificationCreatedPayloadBuilder() {
    NotificationCreatedPayload._defaults(this);
  }

  NotificationCreatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _nid = $v.nid;
      _kind = $v.kind;
      _threadId = $v.threadId;
      _messageId = $v.messageId;
      _targetId = $v.targetId;
      _quiet = $v.quiet;
      _count = $v.count;
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
  void replace(NotificationCreatedPayload other) {
    _$v = other as _$NotificationCreatedPayload;
  }

  @override
  void update(void Function(NotificationCreatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  NotificationCreatedPayload build() => _build();

  _$NotificationCreatedPayload _build() {
    final _$result =
        _$v ??
        _$NotificationCreatedPayload._(
          nid: BuiltValueNullFieldError.checkNotNull(
            nid,
            r'NotificationCreatedPayload',
            'nid',
          ),
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'NotificationCreatedPayload',
            'kind',
          ),
          threadId: threadId,
          messageId: messageId,
          targetId: targetId,
          quiet: BuiltValueNullFieldError.checkNotNull(
            quiet,
            r'NotificationCreatedPayload',
            'quiet',
          ),
          count: BuiltValueNullFieldError.checkNotNull(
            count,
            r'NotificationCreatedPayload',
            'count',
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
