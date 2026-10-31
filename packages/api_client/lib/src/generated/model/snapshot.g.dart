// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'snapshot.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Snapshot extends Snapshot {
  @override
  final int headSeq;
  @override
  final int permissionVersion;
  @override
  final String syncEpoch;
  @override
  final PrincipalRef principal;
  @override
  final bool isOwner;
  @override
  final BuiltList<SpaceView> spaces;
  @override
  final BuiltList<ChannelView> channels;
  @override
  final BuiltList<SidebarState> sidebar;
  @override
  final BuiltList<SidebarChannelState>? sidebarChannels;
  @override
  final BuiltList<UnreadThread> unreadThreads;
  @override
  final BuiltList<RunSummary> activeRuns;
  @override
  final BuiltList<Thread> activeThreads;
  @override
  final BuiltList<AgentPresence> agents;
  @override
  final BuiltList<UserSetting> settings;
  @override
  final BuiltList<DraftRef> drafts;
  @override
  final BuiltList<EvaluatedFlag> features;

  factory _$Snapshot([void Function(SnapshotBuilder)? updates]) =>
      (SnapshotBuilder()..update(updates))._build();

  _$Snapshot._({
    required this.headSeq,
    required this.permissionVersion,
    required this.syncEpoch,
    required this.principal,
    required this.isOwner,
    required this.spaces,
    required this.channels,
    required this.sidebar,
    this.sidebarChannels,
    required this.unreadThreads,
    required this.activeRuns,
    required this.activeThreads,
    required this.agents,
    required this.settings,
    required this.drafts,
    required this.features,
  }) : super._();
  @override
  Snapshot rebuild(void Function(SnapshotBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SnapshotBuilder toBuilder() => SnapshotBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Snapshot &&
        headSeq == other.headSeq &&
        permissionVersion == other.permissionVersion &&
        syncEpoch == other.syncEpoch &&
        principal == other.principal &&
        isOwner == other.isOwner &&
        spaces == other.spaces &&
        channels == other.channels &&
        sidebar == other.sidebar &&
        sidebarChannels == other.sidebarChannels &&
        unreadThreads == other.unreadThreads &&
        activeRuns == other.activeRuns &&
        activeThreads == other.activeThreads &&
        agents == other.agents &&
        settings == other.settings &&
        drafts == other.drafts &&
        features == other.features;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, headSeq.hashCode);
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, isOwner.hashCode);
    _$hash = $jc(_$hash, spaces.hashCode);
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jc(_$hash, sidebar.hashCode);
    _$hash = $jc(_$hash, sidebarChannels.hashCode);
    _$hash = $jc(_$hash, unreadThreads.hashCode);
    _$hash = $jc(_$hash, activeRuns.hashCode);
    _$hash = $jc(_$hash, activeThreads.hashCode);
    _$hash = $jc(_$hash, agents.hashCode);
    _$hash = $jc(_$hash, settings.hashCode);
    _$hash = $jc(_$hash, drafts.hashCode);
    _$hash = $jc(_$hash, features.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Snapshot')
          ..add('headSeq', headSeq)
          ..add('permissionVersion', permissionVersion)
          ..add('syncEpoch', syncEpoch)
          ..add('principal', principal)
          ..add('isOwner', isOwner)
          ..add('spaces', spaces)
          ..add('channels', channels)
          ..add('sidebar', sidebar)
          ..add('sidebarChannels', sidebarChannels)
          ..add('unreadThreads', unreadThreads)
          ..add('activeRuns', activeRuns)
          ..add('activeThreads', activeThreads)
          ..add('agents', agents)
          ..add('settings', settings)
          ..add('drafts', drafts)
          ..add('features', features))
        .toString();
  }
}

class SnapshotBuilder implements Builder<Snapshot, SnapshotBuilder> {
  _$Snapshot? _$v;

  int? _headSeq;
  int? get headSeq => _$this._headSeq;
  set headSeq(int? headSeq) => _$this._headSeq = headSeq;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  bool? _isOwner;
  bool? get isOwner => _$this._isOwner;
  set isOwner(bool? isOwner) => _$this._isOwner = isOwner;

  ListBuilder<SpaceView>? _spaces;
  ListBuilder<SpaceView> get spaces =>
      _$this._spaces ??= ListBuilder<SpaceView>();
  set spaces(ListBuilder<SpaceView>? spaces) => _$this._spaces = spaces;

  ListBuilder<ChannelView>? _channels;
  ListBuilder<ChannelView> get channels =>
      _$this._channels ??= ListBuilder<ChannelView>();
  set channels(ListBuilder<ChannelView>? channels) =>
      _$this._channels = channels;

  ListBuilder<SidebarState>? _sidebar;
  ListBuilder<SidebarState> get sidebar =>
      _$this._sidebar ??= ListBuilder<SidebarState>();
  set sidebar(ListBuilder<SidebarState>? sidebar) => _$this._sidebar = sidebar;

  ListBuilder<SidebarChannelState>? _sidebarChannels;
  ListBuilder<SidebarChannelState> get sidebarChannels =>
      _$this._sidebarChannels ??= ListBuilder<SidebarChannelState>();
  set sidebarChannels(ListBuilder<SidebarChannelState>? sidebarChannels) =>
      _$this._sidebarChannels = sidebarChannels;

  ListBuilder<UnreadThread>? _unreadThreads;
  ListBuilder<UnreadThread> get unreadThreads =>
      _$this._unreadThreads ??= ListBuilder<UnreadThread>();
  set unreadThreads(ListBuilder<UnreadThread>? unreadThreads) =>
      _$this._unreadThreads = unreadThreads;

  ListBuilder<RunSummary>? _activeRuns;
  ListBuilder<RunSummary> get activeRuns =>
      _$this._activeRuns ??= ListBuilder<RunSummary>();
  set activeRuns(ListBuilder<RunSummary>? activeRuns) =>
      _$this._activeRuns = activeRuns;

  ListBuilder<Thread>? _activeThreads;
  ListBuilder<Thread> get activeThreads =>
      _$this._activeThreads ??= ListBuilder<Thread>();
  set activeThreads(ListBuilder<Thread>? activeThreads) =>
      _$this._activeThreads = activeThreads;

  ListBuilder<AgentPresence>? _agents;
  ListBuilder<AgentPresence> get agents =>
      _$this._agents ??= ListBuilder<AgentPresence>();
  set agents(ListBuilder<AgentPresence>? agents) => _$this._agents = agents;

  ListBuilder<UserSetting>? _settings;
  ListBuilder<UserSetting> get settings =>
      _$this._settings ??= ListBuilder<UserSetting>();
  set settings(ListBuilder<UserSetting>? settings) =>
      _$this._settings = settings;

  ListBuilder<DraftRef>? _drafts;
  ListBuilder<DraftRef> get drafts =>
      _$this._drafts ??= ListBuilder<DraftRef>();
  set drafts(ListBuilder<DraftRef>? drafts) => _$this._drafts = drafts;

  ListBuilder<EvaluatedFlag>? _features;
  ListBuilder<EvaluatedFlag> get features =>
      _$this._features ??= ListBuilder<EvaluatedFlag>();
  set features(ListBuilder<EvaluatedFlag>? features) =>
      _$this._features = features;

  SnapshotBuilder() {
    Snapshot._defaults(this);
  }

  SnapshotBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _headSeq = $v.headSeq;
      _permissionVersion = $v.permissionVersion;
      _syncEpoch = $v.syncEpoch;
      _principal = $v.principal.toBuilder();
      _isOwner = $v.isOwner;
      _spaces = $v.spaces.toBuilder();
      _channels = $v.channels.toBuilder();
      _sidebar = $v.sidebar.toBuilder();
      _sidebarChannels = $v.sidebarChannels?.toBuilder();
      _unreadThreads = $v.unreadThreads.toBuilder();
      _activeRuns = $v.activeRuns.toBuilder();
      _activeThreads = $v.activeThreads.toBuilder();
      _agents = $v.agents.toBuilder();
      _settings = $v.settings.toBuilder();
      _drafts = $v.drafts.toBuilder();
      _features = $v.features.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Snapshot other) {
    _$v = other as _$Snapshot;
  }

  @override
  void update(void Function(SnapshotBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Snapshot build() => _build();

  _$Snapshot _build() {
    _$Snapshot _$result;
    try {
      _$result =
          _$v ??
          _$Snapshot._(
            headSeq: BuiltValueNullFieldError.checkNotNull(
              headSeq,
              r'Snapshot',
              'headSeq',
            ),
            permissionVersion: BuiltValueNullFieldError.checkNotNull(
              permissionVersion,
              r'Snapshot',
              'permissionVersion',
            ),
            syncEpoch: BuiltValueNullFieldError.checkNotNull(
              syncEpoch,
              r'Snapshot',
              'syncEpoch',
            ),
            principal: principal.build(),
            isOwner: BuiltValueNullFieldError.checkNotNull(
              isOwner,
              r'Snapshot',
              'isOwner',
            ),
            spaces: spaces.build(),
            channels: channels.build(),
            sidebar: sidebar.build(),
            sidebarChannels: _sidebarChannels?.build(),
            unreadThreads: unreadThreads.build(),
            activeRuns: activeRuns.build(),
            activeThreads: activeThreads.build(),
            agents: agents.build(),
            settings: settings.build(),
            drafts: drafts.build(),
            features: features.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();

        _$failedField = 'spaces';
        spaces.build();
        _$failedField = 'channels';
        channels.build();
        _$failedField = 'sidebar';
        sidebar.build();
        _$failedField = 'sidebarChannels';
        _sidebarChannels?.build();
        _$failedField = 'unreadThreads';
        unreadThreads.build();
        _$failedField = 'activeRuns';
        activeRuns.build();
        _$failedField = 'activeThreads';
        activeThreads.build();
        _$failedField = 'agents';
        agents.build();
        _$failedField = 'settings';
        settings.build();
        _$failedField = 'drafts';
        drafts.build();
        _$failedField = 'features';
        features.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Snapshot',
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
