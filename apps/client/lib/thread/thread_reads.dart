// SPDX-License-Identifier: Apache-2.0

/// The REST reads of the thread view (brief M4-20 Design notes 1, 3 and 4): older pages and
/// ranges through the engine, the position of a linked message, the earlier versions of a
/// message and the templates. Reads are no writes, so they do not go through the outbox
/// (15 1 principle 3 names writes).
library;

import 'package:api_client/api_client.dart'
    show ApiClient, MessageRevision, Template;
import 'package:api_client/api_client.dart' as wire show TemplateScopeEnum;
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

/// An earlier version of a message (API `MessageRevision`).
final class Revision {
  /// Creates the revision.
  const Revision({
    required this.number,
    required this.contentMd,
    required this.editedBy,
    required this.createdAt,
  });

  /// The number, 1 for the first text.
  final int number;

  /// The text of this version.
  final String contentMd;

  /// Who wrote the edit that replaced it.
  final PrincipalId editedBy;

  /// When this version was written.
  final DateTime createdAt;
}

/// A template (API `Template`).
final class TemplateItem {
  /// Creates the template.
  const TemplateItem({
    required this.id,
    required this.name,
    required this.contentMd,
    required this.personal,
  });

  /// The template ID.
  final String id;

  /// The name shown in the menu.
  final String name;

  /// The text inserted into the composer.
  final String contentMd;

  /// Whether it is a personal template (else a template of the channel).
  final bool personal;
}

/// The personal templates and the templates of a channel (01 3.3).
final class TemplateLists {
  /// Creates the lists.
  const TemplateLists({required this.personal, required this.channel});

  /// `listMyTemplates`.
  final List<TemplateItem> personal;

  /// `listTemplates` of the channel.
  final List<TemplateItem> channel;
}

/// The reads of the thread view; tests replace it.
abstract interface class ThreadReads {
  /// Loads the page of [thread] before [before] into the local database; returns whether
  /// older messages exist. Throws what the read throws.
  Future<bool> loadOlder(ThreadId thread, ThreadSeq before);

  /// Loads the messages of [thread] from [from] to [to] into the local database.
  Future<void> loadRange(ThreadId thread, ThreadSeq from, ThreadSeq to);

  /// The `thread_seq` of message [id] (`getMessage`), or null when it is not visible.
  Future<ThreadSeq?> positionOf(MessageId id);

  /// `listMessageRevisions` of [id], oldest first.
  Future<List<Revision>> revisions(MessageId id);

  /// `listMyTemplates` and `listTemplates` of [channelId].
  Future<TemplateLists> templates(ChannelId channelId);
}

/// [ThreadReads] over the engine's catch-up and the session's REST client.
///
/// Example: `final reads = EngineThreadReads(engine);`
final class EngineThreadReads implements ThreadReads {
  /// Creates the reads of [engine].
  EngineThreadReads(this._engine);

  final SyncEngine _engine;

  ApiClient get _api => _engine.api;

  @override
  Future<bool> loadOlder(ThreadId thread, ThreadSeq before) =>
      _engine.catchUp.loadOlderMessages(thread, before: before);

  @override
  Future<void> loadRange(ThreadId thread, ThreadSeq from, ThreadSeq to) =>
      _engine.catchUp.loadMessageRange(thread, from, to);

  @override
  Future<ThreadSeq?> positionOf(MessageId id) async {
    final bundle = await ApiFetcher(_api).message(id);
    return bundle.message.threadSeq;
  }

  @override
  Future<List<Revision>> revisions(MessageId id) async {
    final list = (await _api.getMessagesApi().listMessageRevisions(
      messageId: id.value,
    )).data;
    return [
      for (final r in list?.items ?? const <MessageRevision>[])
        Revision(
          number: r.revision,
          contentMd: r.contentMd,
          editedBy: PrincipalId.parse(r.editedBy),
          createdAt: r.createdAt,
        ),
    ];
  }

  @override
  Future<TemplateLists> templates(ChannelId channelId) async {
    final mine = (await _api.getMeApi().listMyTemplates()).data;
    final channel = (await _api.getChannelsApi().listTemplates(
      channelId: channelId.value,
    )).data;
    TemplateItem item(Template t) => TemplateItem(
      id: t.id,
      name: t.name,
      contentMd: t.contentMd,
      personal: t.scope == wire.TemplateScopeEnum.personal,
    );
    return TemplateLists(
      personal: [for (final t in mine?.items ?? const <Template>[]) item(t)],
      channel: [for (final t in channel?.items ?? const <Template>[]) item(t)],
    );
  }
}
