// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'providers.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The REST reads of the agent interface; tests override it.

@ProviderFor(agentReads)
final agentReadsProvider = AgentReadsProvider._();

/// The REST reads of the agent interface; tests override it.

final class AgentReadsProvider
    extends $FunctionalProvider<AgentReads, AgentReads, AgentReads>
    with $Provider<AgentReads> {
  /// The REST reads of the agent interface; tests override it.
  AgentReadsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'agentReadsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$agentReadsHash();

  @$internal
  @override
  $ProviderElement<AgentReads> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AgentReads create(Ref ref) {
    return agentReads(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AgentReads value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AgentReads>(value),
    );
  }
}

String _$agentReadsHash() => r'aa4b54cb65194884ce5730c1c4ff23d49ef40c83';

/// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
/// of `run.state_changed`, 04 11.3).

@ProviderFor(threadRuns)
final threadRunsProvider = ThreadRunsFamily._();

/// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
/// of `run.state_changed`, 04 11.3).

final class ThreadRunsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<RunSummary>>,
          List<RunSummary>,
          Stream<List<RunSummary>>
        >
    with $FutureModifier<List<RunSummary>>, $StreamProvider<List<RunSummary>> {
  /// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
  /// of `run.state_changed`, 04 11.3).
  ThreadRunsProvider._({
    required ThreadRunsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadRunsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadRunsHash();

  @override
  String toString() {
    return r'threadRunsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<RunSummary>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<RunSummary>> create(Ref ref) {
    final argument = this.argument as String;
    return threadRuns(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadRunsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadRunsHash() => r'0ea2320512a0457db14396508ed6cef6596e2ebf';

/// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
/// of `run.state_changed`, 04 11.3).

final class ThreadRunsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<RunSummary>>, String> {
  ThreadRunsFamily._()
    : super(
        retry: null,
        name: r'threadRunsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
  /// of `run.state_changed`, 04 11.3).

  ThreadRunsProvider call(String threadId) =>
      ThreadRunsProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadRunsProvider';
}

/// The run source of the signed-in engine, or null; tests override it.

@ProviderFor(runSource)
final runSourceProvider = RunSourceProvider._();

/// The run source of the signed-in engine, or null; tests override it.

final class RunSourceProvider
    extends $FunctionalProvider<RunSource?, RunSource?, RunSource?>
    with $Provider<RunSource?> {
  /// The run source of the signed-in engine, or null; tests override it.
  RunSourceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runSourceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runSourceHash();

  @$internal
  @override
  $ProviderElement<RunSource?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  RunSource? create(Ref ref) {
    return runSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(RunSource? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<RunSource?>(value),
    );
  }
}

String _$runSourceHash() => r'09c4a6b9c5e35b6a7c33ea679f63a9b6dbe3a8c3';

/// The document of [runId] in [threadId] while the live box shows it.

@ProviderFor(runDocument)
final runDocumentProvider = RunDocumentFamily._();

/// The document of [runId] in [threadId] while the live box shows it.

final class RunDocumentProvider
    extends
        $FunctionalProvider<
          AsyncValue<RunDocument?>,
          RunDocument?,
          Stream<RunDocument?>
        >
    with $FutureModifier<RunDocument?>, $StreamProvider<RunDocument?> {
  /// The document of [runId] in [threadId] while the live box shows it.
  RunDocumentProvider._({
    required RunDocumentFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: null,
         name: r'runDocumentProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$runDocumentHash();

  @override
  String toString() {
    return r'runDocumentProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $StreamProviderElement<RunDocument?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<RunDocument?> create(Ref ref) {
    final argument = this.argument as (String, String);
    return runDocument(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is RunDocumentProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$runDocumentHash() => r'9360f96cd5f045bdd91f37e5c2e9aa49f7016b87';

/// The document of [runId] in [threadId] while the live box shows it.

final class RunDocumentFamily extends $Family
    with $FunctionalFamilyOverride<Stream<RunDocument?>, (String, String)> {
  RunDocumentFamily._()
    : super(
        retry: null,
        name: r'runDocumentProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The document of [runId] in [threadId] while the live box shows it.

  RunDocumentProvider call(String runId, String threadId) =>
      RunDocumentProvider._(argument: (runId, threadId), from: this);

  @override
  String toString() => r'runDocumentProvider';
}

/// The document of the ended run [runId], loaded once when its collapsed line is expanded
/// (`getRun`, 01 6.2; brief M4-21 Design note 1).

@ProviderFor(endedRunDocument)
final endedRunDocumentProvider = EndedRunDocumentFamily._();

/// The document of the ended run [runId], loaded once when its collapsed line is expanded
/// (`getRun`, 01 6.2; brief M4-21 Design note 1).

final class EndedRunDocumentProvider
    extends
        $FunctionalProvider<
          AsyncValue<RunDocument?>,
          RunDocument?,
          FutureOr<RunDocument?>
        >
    with $FutureModifier<RunDocument?>, $FutureProvider<RunDocument?> {
  /// The document of the ended run [runId], loaded once when its collapsed line is expanded
  /// (`getRun`, 01 6.2; brief M4-21 Design note 1).
  EndedRunDocumentProvider._({
    required EndedRunDocumentFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'endedRunDocumentProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$endedRunDocumentHash();

  @override
  String toString() {
    return r'endedRunDocumentProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<RunDocument?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<RunDocument?> create(Ref ref) {
    final argument = this.argument as String;
    return endedRunDocument(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is EndedRunDocumentProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$endedRunDocumentHash() => r'62c7a4450a6a56225312187d019084b6f2d2e2f4';

/// The document of the ended run [runId], loaded once when its collapsed line is expanded
/// (`getRun`, 01 6.2; brief M4-21 Design note 1).

final class EndedRunDocumentFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<RunDocument?>, String> {
  EndedRunDocumentFamily._()
    : super(
        retry: null,
        name: r'endedRunDocumentProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The document of the ended run [runId], loaded once when its collapsed line is expanded
  /// (`getRun`, 01 6.2; brief M4-21 Design note 1).

  EndedRunDocumentProvider call(String runId) =>
      EndedRunDocumentProvider._(argument: runId, from: this);

  @override
  String toString() => r'endedRunDocumentProvider';
}

/// The components of the card [messageId] at its [version]: the echo `message.updated`
/// raises the version, which reads them again.

@ProviderFor(messageComponents)
final messageComponentsProvider = MessageComponentsFamily._();

/// The components of the card [messageId] at its [version]: the echo `message.updated`
/// raises the version, which reads them again.

final class MessageComponentsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<MessageComponent>>,
          List<MessageComponent>,
          FutureOr<List<MessageComponent>>
        >
    with
        $FutureModifier<List<MessageComponent>>,
        $FutureProvider<List<MessageComponent>> {
  /// The components of the card [messageId] at its [version]: the echo `message.updated`
  /// raises the version, which reads them again.
  MessageComponentsProvider._({
    required MessageComponentsFamily super.from,
    required (String, int) super.argument,
  }) : super(
         retry: null,
         name: r'messageComponentsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$messageComponentsHash();

  @override
  String toString() {
    return r'messageComponentsProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $FutureProviderElement<List<MessageComponent>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<MessageComponent>> create(Ref ref) {
    final argument = this.argument as (String, int);
    return messageComponents(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is MessageComponentsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$messageComponentsHash() => r'882f38eea6735a37957306ebdb7725557b1c9982';

/// The components of the card [messageId] at its [version]: the echo `message.updated`
/// raises the version, which reads them again.

final class MessageComponentsFamily extends $Family
    with
        $FunctionalFamilyOverride<
          FutureOr<List<MessageComponent>>,
          (String, int)
        > {
  MessageComponentsFamily._()
    : super(
        retry: null,
        name: r'messageComponentsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The components of the card [messageId] at its [version]: the echo `message.updated`
  /// raises the version, which reads them again.

  MessageComponentsProvider call(String messageId, int version) =>
      MessageComponentsProvider._(argument: (messageId, version), from: this);

  @override
  String toString() => r'messageComponentsProvider';
}

/// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
/// `commands` table (15 6.1: suggestions work offline).

@ProviderFor(channelCommands)
final channelCommandsProvider = ChannelCommandsFamily._();

/// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
/// `commands` table (15 6.1: suggestions work offline).

final class ChannelCommandsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Command>>,
          List<Command>,
          Stream<List<Command>>
        >
    with $FutureModifier<List<Command>>, $StreamProvider<List<Command>> {
  /// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
  /// `commands` table (15 6.1: suggestions work offline).
  ChannelCommandsProvider._({
    required ChannelCommandsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelCommandsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelCommandsHash();

  @override
  String toString() {
    return r'channelCommandsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<List<Command>> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<List<Command>> create(Ref ref) {
    final argument = this.argument as String;
    return channelCommands(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelCommandsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelCommandsHash() => r'55a946c00cfaa4dc3b9e6a7bf116a049f2a15f72';

/// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
/// `commands` table (15 6.1: suggestions work offline).

final class ChannelCommandsFamily extends $Family
    with $FunctionalFamilyOverride<Stream<List<Command>>, String> {
  ChannelCommandsFamily._()
    : super(
        retry: null,
        name: r'channelCommandsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
  /// `commands` table (15 6.1: suggestions work offline).

  ChannelCommandsProvider call(String channelId) =>
      ChannelCommandsProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelCommandsProvider';
}

/// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).

@ProviderFor(agentPresence)
final agentPresenceProvider = AgentPresenceFamily._();

/// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).

final class AgentPresenceProvider
    extends
        $FunctionalProvider<
          AsyncValue<AgentPresence?>,
          AgentPresence?,
          Stream<AgentPresence?>
        >
    with $FutureModifier<AgentPresence?>, $StreamProvider<AgentPresence?> {
  /// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).
  AgentPresenceProvider._({
    required AgentPresenceFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'agentPresenceProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$agentPresenceHash();

  @override
  String toString() {
    return r'agentPresenceProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $StreamProviderElement<AgentPresence?> $createElement(
    $ProviderPointer pointer,
  ) => $StreamProviderElement(pointer);

  @override
  Stream<AgentPresence?> create(Ref ref) {
    final argument = this.argument as String;
    return agentPresence(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AgentPresenceProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$agentPresenceHash() => r'843237b37a0ac4bb4845c1cc3856109d80931673';

/// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).

final class AgentPresenceFamily extends $Family
    with $FunctionalFamilyOverride<Stream<AgentPresence?>, String> {
  AgentPresenceFamily._()
    : super(
        retry: null,
        name: r'agentPresenceProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).

  AgentPresenceProvider call(String agentId) =>
      AgentPresenceProvider._(argument: agentId, from: this);

  @override
  String toString() => r'agentPresenceProvider';
}

/// The agent [agentId] as `getAgent` answers it.

@ProviderFor(agentInfo)
final agentInfoProvider = AgentInfoFamily._();

/// The agent [agentId] as `getAgent` answers it.

final class AgentInfoProvider
    extends
        $FunctionalProvider<
          AsyncValue<AgentInfo?>,
          AgentInfo?,
          FutureOr<AgentInfo?>
        >
    with $FutureModifier<AgentInfo?>, $FutureProvider<AgentInfo?> {
  /// The agent [agentId] as `getAgent` answers it.
  AgentInfoProvider._({
    required AgentInfoFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'agentInfoProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$agentInfoHash();

  @override
  String toString() {
    return r'agentInfoProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<AgentInfo?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<AgentInfo?> create(Ref ref) {
    final argument = this.argument as String;
    return agentInfo(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AgentInfoProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$agentInfoHash() => r'844647e1f3930223fda48a6d22acfa96994f8a50';

/// The agent [agentId] as `getAgent` answers it.

final class AgentInfoFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<AgentInfo?>, String> {
  AgentInfoFamily._()
    : super(
        retry: null,
        name: r'agentInfoProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The agent [agentId] as `getAgent` answers it.

  AgentInfoProvider call(String agentId) =>
      AgentInfoProvider._(argument: agentId, from: this);

  @override
  String toString() => r'agentInfoProvider';
}

/// The agent sessions of [threadId], newest first.

@ProviderFor(agentSessions)
final agentSessionsProvider = AgentSessionsFamily._();

/// The agent sessions of [threadId], newest first.

final class AgentSessionsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<AgentSessionView>>,
          List<AgentSessionView>,
          FutureOr<List<AgentSessionView>>
        >
    with
        $FutureModifier<List<AgentSessionView>>,
        $FutureProvider<List<AgentSessionView>> {
  /// The agent sessions of [threadId], newest first.
  AgentSessionsProvider._({
    required AgentSessionsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'agentSessionsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$agentSessionsHash();

  @override
  String toString() {
    return r'agentSessionsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<AgentSessionView>> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<AgentSessionView>> create(Ref ref) {
    final argument = this.argument as String;
    return agentSessions(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AgentSessionsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$agentSessionsHash() => r'473a347a2efc739b0fa36ec6324f22d4b01b0c41';

/// The agent sessions of [threadId], newest first.

final class AgentSessionsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<AgentSessionView>>, String> {
  AgentSessionsFamily._()
    : super(
        retry: null,
        name: r'agentSessionsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The agent sessions of [threadId], newest first.

  AgentSessionsProvider call(String threadId) =>
      AgentSessionsProvider._(argument: threadId, from: this);

  @override
  String toString() => r'agentSessionsProvider';
}

/// The cost totals of [threadId].

@ProviderFor(threadCosts)
final threadCostsProvider = ThreadCostsFamily._();

/// The cost totals of [threadId].

final class ThreadCostsProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<ThreadCost>?>,
          List<ThreadCost>?,
          FutureOr<List<ThreadCost>?>
        >
    with
        $FutureModifier<List<ThreadCost>?>,
        $FutureProvider<List<ThreadCost>?> {
  /// The cost totals of [threadId].
  ThreadCostsProvider._({
    required ThreadCostsFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadCostsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadCostsHash();

  @override
  String toString() {
    return r'threadCostsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<ThreadCost>?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<List<ThreadCost>?> create(Ref ref) {
    final argument = this.argument as String;
    return threadCosts(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadCostsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadCostsHash() => r'e675e54326b5d401d1fc407e2d0695b204eed1a0';

/// The cost totals of [threadId].

final class ThreadCostsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<ThreadCost>?>, String> {
  ThreadCostsFamily._()
    : super(
        retry: null,
        name: r'threadCostsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The cost totals of [threadId].

  ThreadCostsProvider call(String threadId) =>
      ThreadCostsProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadCostsProvider';
}

/// The number of runs of [threadId].

@ProviderFor(threadRunCount)
final threadRunCountProvider = ThreadRunCountFamily._();

/// The number of runs of [threadId].

final class ThreadRunCountProvider
    extends
        $FunctionalProvider<
          AsyncValue<({int count, bool more})?>,
          ({int count, bool more})?,
          FutureOr<({int count, bool more})?>
        >
    with
        $FutureModifier<({int count, bool more})?>,
        $FutureProvider<({int count, bool more})?> {
  /// The number of runs of [threadId].
  ThreadRunCountProvider._({
    required ThreadRunCountFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'threadRunCountProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$threadRunCountHash();

  @override
  String toString() {
    return r'threadRunCountProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<({int count, bool more})?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<({int count, bool more})?> create(Ref ref) {
    final argument = this.argument as String;
    return threadRunCount(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ThreadRunCountProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$threadRunCountHash() => r'be93f40060f257de9aa69ab3acf18d9fbe3ffc44';

/// The number of runs of [threadId].

final class ThreadRunCountFamily extends $Family
    with
        $FunctionalFamilyOverride<FutureOr<({int count, bool more})?>, String> {
  ThreadRunCountFamily._()
    : super(
        retry: null,
        name: r'threadRunCountProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The number of runs of [threadId].

  ThreadRunCountProvider call(String threadId) =>
      ThreadRunCountProvider._(argument: threadId, from: this);

  @override
  String toString() => r'threadRunCountProvider';
}

/// The project binding of [channelId].

@ProviderFor(channelProject)
final channelProjectProvider = ChannelProjectFamily._();

/// The project binding of [channelId].

final class ChannelProjectProvider
    extends $FunctionalProvider<AsyncValue<String?>, String?, FutureOr<String?>>
    with $FutureModifier<String?>, $FutureProvider<String?> {
  /// The project binding of [channelId].
  ChannelProjectProvider._({
    required ChannelProjectFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'channelProjectProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$channelProjectHash();

  @override
  String toString() {
    return r'channelProjectProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<String?> create(Ref ref) {
    final argument = this.argument as String;
    return channelProject(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is ChannelProjectProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$channelProjectHash() => r'4c68a3e0885f45e0d33f5a19dc84f86a46fad858';

/// The project binding of [channelId].

final class ChannelProjectFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<String?>, String> {
  ChannelProjectFamily._()
    : super(
        retry: null,
        name: r'channelProjectProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// The project binding of [channelId].

  ChannelProjectProvider call(String channelId) =>
      ChannelProjectProvider._(argument: channelId, from: this);

  @override
  String toString() => r'channelProjectProvider';
}

/// The runs whose box the person expanded or collapsed by hand, by run ID: true expanded,
/// false collapsed (local state; the keyboard binding is M4-24). It lasts while the app runs.

@ProviderFor(RunExpansion)
final runExpansionProvider = RunExpansionProvider._();

/// The runs whose box the person expanded or collapsed by hand, by run ID: true expanded,
/// false collapsed (local state; the keyboard binding is M4-24). It lasts while the app runs.
final class RunExpansionProvider
    extends $NotifierProvider<RunExpansion, Map<String, bool>> {
  /// The runs whose box the person expanded or collapsed by hand, by run ID: true expanded,
  /// false collapsed (local state; the keyboard binding is M4-24). It lasts while the app runs.
  RunExpansionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'runExpansionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$runExpansionHash();

  @$internal
  @override
  RunExpansion create() => RunExpansion();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Map<String, bool> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Map<String, bool>>(value),
    );
  }
}

String _$runExpansionHash() => r'097bb23c94be51801ef2b6013c09e9523e8afee3';

/// The runs whose box the person expanded or collapsed by hand, by run ID: true expanded,
/// false collapsed (local state; the keyboard binding is M4-24). It lasts while the app runs.

abstract class _$RunExpansion extends $Notifier<Map<String, bool>> {
  Map<String, bool> build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<Map<String, bool>, Map<String, bool>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<Map<String, bool>, Map<String, bool>>,
              Map<String, bool>,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
