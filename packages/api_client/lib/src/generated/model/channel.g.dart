// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Channel extends Channel {
  @override
  final String id;
  @override
  final String? spaceId;
  @override
  final ChannelKind kind;
  @override
  final String name;
  @override
  final String topic;
  @override
  final String sortKey;
  @override
  final ChannelOperator operator_;
  @override
  final ForumSort? forumDefaultSort;
  @override
  final NotificationLevel? defaultNotificationLevel;
  @override
  final bool readOnly;
  @override
  final bool projectBound;
  @override
  final String? mainThreadId;
  @override
  final BuiltList<String>? dmParticipantIds;
  @override
  final DateTime? archivedAt;
  @override
  final DateTime? deletedAt;
  @override
  final int version;
  @override
  final int settingsVersion;
  @override
  final DateTime createdAt;

  factory _$Channel([void Function(ChannelBuilder)? updates]) =>
      (ChannelBuilder()..update(updates))._build();

  _$Channel._({
    required this.id,
    this.spaceId,
    required this.kind,
    required this.name,
    required this.topic,
    required this.sortKey,
    required this.operator_,
    this.forumDefaultSort,
    this.defaultNotificationLevel,
    required this.readOnly,
    required this.projectBound,
    this.mainThreadId,
    this.dmParticipantIds,
    this.archivedAt,
    this.deletedAt,
    required this.version,
    required this.settingsVersion,
    required this.createdAt,
  }) : super._();
  @override
  Channel rebuild(void Function(ChannelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelBuilder toBuilder() => ChannelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Channel &&
        id == other.id &&
        spaceId == other.spaceId &&
        kind == other.kind &&
        name == other.name &&
        topic == other.topic &&
        sortKey == other.sortKey &&
        operator_ == other.operator_ &&
        forumDefaultSort == other.forumDefaultSort &&
        defaultNotificationLevel == other.defaultNotificationLevel &&
        readOnly == other.readOnly &&
        projectBound == other.projectBound &&
        mainThreadId == other.mainThreadId &&
        dmParticipantIds == other.dmParticipantIds &&
        archivedAt == other.archivedAt &&
        deletedAt == other.deletedAt &&
        version == other.version &&
        settingsVersion == other.settingsVersion &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, spaceId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, topic.hashCode);
    _$hash = $jc(_$hash, sortKey.hashCode);
    _$hash = $jc(_$hash, operator_.hashCode);
    _$hash = $jc(_$hash, forumDefaultSort.hashCode);
    _$hash = $jc(_$hash, defaultNotificationLevel.hashCode);
    _$hash = $jc(_$hash, readOnly.hashCode);
    _$hash = $jc(_$hash, projectBound.hashCode);
    _$hash = $jc(_$hash, mainThreadId.hashCode);
    _$hash = $jc(_$hash, dmParticipantIds.hashCode);
    _$hash = $jc(_$hash, archivedAt.hashCode);
    _$hash = $jc(_$hash, deletedAt.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, settingsVersion.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Channel')
          ..add('id', id)
          ..add('spaceId', spaceId)
          ..add('kind', kind)
          ..add('name', name)
          ..add('topic', topic)
          ..add('sortKey', sortKey)
          ..add('operator_', operator_)
          ..add('forumDefaultSort', forumDefaultSort)
          ..add('defaultNotificationLevel', defaultNotificationLevel)
          ..add('readOnly', readOnly)
          ..add('projectBound', projectBound)
          ..add('mainThreadId', mainThreadId)
          ..add('dmParticipantIds', dmParticipantIds)
          ..add('archivedAt', archivedAt)
          ..add('deletedAt', deletedAt)
          ..add('version', version)
          ..add('settingsVersion', settingsVersion)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class ChannelBuilder implements Builder<Channel, ChannelBuilder> {
  _$Channel? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _spaceId;
  String? get spaceId => _$this._spaceId;
  set spaceId(String? spaceId) => _$this._spaceId = spaceId;

  ChannelKind? _kind;
  ChannelKind? get kind => _$this._kind;
  set kind(ChannelKind? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _topic;
  String? get topic => _$this._topic;
  set topic(String? topic) => _$this._topic = topic;

  String? _sortKey;
  String? get sortKey => _$this._sortKey;
  set sortKey(String? sortKey) => _$this._sortKey = sortKey;

  ChannelOperatorBuilder? _operator_;
  ChannelOperatorBuilder get operator_ =>
      _$this._operator_ ??= ChannelOperatorBuilder();
  set operator_(ChannelOperatorBuilder? operator_) =>
      _$this._operator_ = operator_;

  ForumSort? _forumDefaultSort;
  ForumSort? get forumDefaultSort => _$this._forumDefaultSort;
  set forumDefaultSort(ForumSort? forumDefaultSort) =>
      _$this._forumDefaultSort = forumDefaultSort;

  NotificationLevel? _defaultNotificationLevel;
  NotificationLevel? get defaultNotificationLevel =>
      _$this._defaultNotificationLevel;
  set defaultNotificationLevel(NotificationLevel? defaultNotificationLevel) =>
      _$this._defaultNotificationLevel = defaultNotificationLevel;

  bool? _readOnly;
  bool? get readOnly => _$this._readOnly;
  set readOnly(bool? readOnly) => _$this._readOnly = readOnly;

  bool? _projectBound;
  bool? get projectBound => _$this._projectBound;
  set projectBound(bool? projectBound) => _$this._projectBound = projectBound;

  String? _mainThreadId;
  String? get mainThreadId => _$this._mainThreadId;
  set mainThreadId(String? mainThreadId) => _$this._mainThreadId = mainThreadId;

  ListBuilder<String>? _dmParticipantIds;
  ListBuilder<String> get dmParticipantIds =>
      _$this._dmParticipantIds ??= ListBuilder<String>();
  set dmParticipantIds(ListBuilder<String>? dmParticipantIds) =>
      _$this._dmParticipantIds = dmParticipantIds;

  DateTime? _archivedAt;
  DateTime? get archivedAt => _$this._archivedAt;
  set archivedAt(DateTime? archivedAt) => _$this._archivedAt = archivedAt;

  DateTime? _deletedAt;
  DateTime? get deletedAt => _$this._deletedAt;
  set deletedAt(DateTime? deletedAt) => _$this._deletedAt = deletedAt;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  int? _settingsVersion;
  int? get settingsVersion => _$this._settingsVersion;
  set settingsVersion(int? settingsVersion) =>
      _$this._settingsVersion = settingsVersion;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  ChannelBuilder() {
    Channel._defaults(this);
  }

  ChannelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _spaceId = $v.spaceId;
      _kind = $v.kind;
      _name = $v.name;
      _topic = $v.topic;
      _sortKey = $v.sortKey;
      _operator_ = $v.operator_.toBuilder();
      _forumDefaultSort = $v.forumDefaultSort;
      _defaultNotificationLevel = $v.defaultNotificationLevel;
      _readOnly = $v.readOnly;
      _projectBound = $v.projectBound;
      _mainThreadId = $v.mainThreadId;
      _dmParticipantIds = $v.dmParticipantIds?.toBuilder();
      _archivedAt = $v.archivedAt;
      _deletedAt = $v.deletedAt;
      _version = $v.version;
      _settingsVersion = $v.settingsVersion;
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Channel other) {
    _$v = other as _$Channel;
  }

  @override
  void update(void Function(ChannelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Channel build() => _build();

  _$Channel _build() {
    _$Channel _$result;
    try {
      _$result =
          _$v ??
          _$Channel._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Channel', 'id'),
            spaceId: spaceId,
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'Channel',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'Channel',
              'name',
            ),
            topic: BuiltValueNullFieldError.checkNotNull(
              topic,
              r'Channel',
              'topic',
            ),
            sortKey: BuiltValueNullFieldError.checkNotNull(
              sortKey,
              r'Channel',
              'sortKey',
            ),
            operator_: operator_.build(),
            forumDefaultSort: forumDefaultSort,
            defaultNotificationLevel: defaultNotificationLevel,
            readOnly: BuiltValueNullFieldError.checkNotNull(
              readOnly,
              r'Channel',
              'readOnly',
            ),
            projectBound: BuiltValueNullFieldError.checkNotNull(
              projectBound,
              r'Channel',
              'projectBound',
            ),
            mainThreadId: mainThreadId,
            dmParticipantIds: _dmParticipantIds?.build(),
            archivedAt: archivedAt,
            deletedAt: deletedAt,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'Channel',
              'version',
            ),
            settingsVersion: BuiltValueNullFieldError.checkNotNull(
              settingsVersion,
              r'Channel',
              'settingsVersion',
            ),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Channel',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'operator_';
        operator_.build();

        _$failedField = 'dmParticipantIds';
        _dmParticipantIds?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Channel',
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
