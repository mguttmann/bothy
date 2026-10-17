// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for German (`de`).
class AppLocalizationsDe extends AppLocalizations {
  AppLocalizationsDe([String locale = 'de']) : super(locale);

  @override
  String get triggerStateReceived => 'Empfangen';

  @override
  String get triggerStateQueued => 'In Warteschlange';

  @override
  String get triggerStateWorking => 'Wird bearbeitet';

  @override
  String get triggerStateDone => 'Erledigt';

  @override
  String get triggerStateFailed => 'Fehlgeschlagen';

  @override
  String get triggerStateFollowupDelivered => 'Nachtrag zugestellt';

  @override
  String triggerStateFailedWithReason(String reason) {
    return 'Fehlgeschlagen: $reason';
  }

  @override
  String get liveBoxStateQueued => 'In Warteschlange';

  @override
  String get liveBoxStateRunning => 'Läuft';

  @override
  String get liveBoxStateWaitingForYou => 'Wartet auf dich';

  @override
  String get liveBoxStateCompleted => 'Erledigt';

  @override
  String get liveBoxStateFailed => 'Fehlgeschlagen';

  @override
  String get liveBoxStateCancelled => 'Abgebrochen';

  @override
  String liveBoxStateWaitingFor(String name) {
    return 'Wartet auf $name';
  }

  @override
  String get liveBoxReasonBillingModeMismatch => 'Backend passt nicht';

  @override
  String get liveBoxReasonAccessRevoked => 'Zugriff entzogen';

  @override
  String get liveBoxReasonStale => 'Abgebrochen nach Stillstand';

  @override
  String get liveBoxReasonAdapterError => 'Fehler im Adapter';

  @override
  String get liveBoxReasonCancelTimeout =>
      'Adapter hat den Abbruch nicht bestätigt';

  @override
  String get liveBoxReasonProjectNotReady => 'Projekt nicht bereit';

  @override
  String liveBoxReasonUserCancelled(String name) {
    return 'Von $name abgebrochen';
  }

  @override
  String get liveBoxStop => 'Stopp';

  @override
  String liveBoxEarlierSteps(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count frühere Schritte',
      one: '1 früherer Schritt',
    );
    return '$_temp0';
  }

  @override
  String liveBoxStall(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: 'Seit $minutes Minuten kein Fortschritt',
      one: 'Seit 1 Minute kein Fortschritt',
    );
    return '$_temp0';
  }

  @override
  String get accessNotFoundOrNoAccess => 'Nicht gefunden oder kein Zugriff';

  @override
  String get accessNoAccessAnymore => 'Kein Zugriff mehr';

  @override
  String get messageSending => 'Wird gesendet';

  @override
  String get messageSendingDelayed => 'Senden verzögert';

  @override
  String get connectionOffline => 'Offline';

  @override
  String get relativeDayToday => 'Heute';

  @override
  String get relativeDayYesterday => 'Gestern';

  @override
  String get problemValidationFailed => 'Einige Eingaben sind ungültig';

  @override
  String get problemMalformedRequest =>
      'Die Anfrage konnte nicht gelesen werden';

  @override
  String get problemCursorInvalid =>
      'Die Listenposition ist nicht mehr gültig. Lade die Liste neu';

  @override
  String get problemWebhookHostNotAllowed =>
      'Das Webhook-Ziel steht nicht auf der Erlaubnisliste';

  @override
  String get problemUnauthenticated => 'Bitte melde dich an';

  @override
  String get problemTokenExpired => 'Deine Anmeldung ist abgelaufen';

  @override
  String get problemSessionRevoked => 'Diese Sitzung wurde beendet';

  @override
  String get problemStepUpRequired => 'Bitte bestätige mit deinem Gerät';

  @override
  String get problemUseDpopNonce => 'Bitte versuche es noch einmal';

  @override
  String get problemInvalidDpopProof =>
      'Der Nachweis des Geräts wurde nicht akzeptiert';

  @override
  String get problemForbidden => 'Das darfst du nicht';

  @override
  String get problemScopeMissing => 'Der App fehlt ein nötiger Scope';

  @override
  String get problemDevicePending => 'Dieses Gerät wartet auf Bestätigung';

  @override
  String get problemChannelReadOnly => 'Dieser Kanal ist schreibgeschützt';

  @override
  String get problemFeatureDisabled => 'Diese Funktion ist ausgeschaltet';

  @override
  String get problemAgentInvocationDenied =>
      'Der Agent kann hier nicht gestartet werden';

  @override
  String get problemTerminalDenied =>
      'Das Terminal kann hier nicht geöffnet werden';

  @override
  String get problemInvocationTokenInvalid =>
      'Die Aufrufmarke des Agenten ist ungültig';

  @override
  String get problemNotFound => 'Nicht gefunden oder kein Zugriff';

  @override
  String get problemMethodNotAllowed => 'Diese Anfrage wird nicht unterstützt';

  @override
  String get problemConflict => 'Das widerspricht dem aktuellen Stand';

  @override
  String get problemRequestInProgress => 'Die Anfrage läuft noch';

  @override
  String get problemI1Violation =>
      'Mit dieser Änderung könnten andere einen persönlichen Agenten nutzen';

  @override
  String get problemI6Violation =>
      'Nur wer die persönliche LLM-Anmeldung hält, darf auf diesem Rechner ein Terminal öffnen';

  @override
  String get problemLastOwner => 'Mindestens ein aktiver Owner muss bleiben';

  @override
  String get problemAlreadyStarted => 'Der Lauf hat schon begonnen';

  @override
  String get problemBillingModeMismatch =>
      'Das Backend passt nicht zum Abrechnungsmodus';

  @override
  String get problemToolClassMismatch =>
      'Die gemeldete Werkzeugklasse liegt über der erlaubten';

  @override
  String get problemHostUnavailable => 'Der Rechner ist nicht erreichbar';

  @override
  String get problemFingerprintMismatch =>
      'Der Fingerabdruck passt nicht zum Rechner';

  @override
  String get problemInvalidStateTransition =>
      'Diese Änderung ist im aktuellen Zustand nicht möglich';

  @override
  String get problemBaseVersionConflict =>
      'Der Lauf hat sich inzwischen geändert';

  @override
  String get problemComponentUsed =>
      'Dieses Element wurde schon benutzt, ist abgelaufen oder ausgeschaltet';

  @override
  String get problemOperatorMismatch =>
      'Der Zielkanal hat einen anderen Bediener';

  @override
  String get problemFileNotReady => 'Die Datei ist noch nicht bereit';

  @override
  String get problemFileBlocked =>
      'Die Datei ist gesperrt und wird nie ausgeliefert';

  @override
  String get problemTitleManual => 'Der Titel wurde von Hand gesetzt';

  @override
  String get problemResyncRequired => 'Ein vollständiges Neuladen ist nötig';

  @override
  String get problemMessageDeleted => 'Die Nachricht wurde gelöscht';

  @override
  String get problemGone => 'Das ist abgelaufen';

  @override
  String get problemVersionConflict =>
      'Jemand anderes hat das inzwischen geändert';

  @override
  String get problemPayloadTooLarge => 'Der Inhalt ist zu groß';

  @override
  String get problemUnsupportedMediaType => 'Dieser Dateityp ist nicht erlaubt';

  @override
  String get problemRangeNotSatisfiable =>
      'Der angefragte Ausschnitt liegt außerhalb der Datei';

  @override
  String get problemIdempotencyKeyReuse =>
      'Die Anfrage wurde mit anderem Inhalt wiederholt';

  @override
  String get problemPreconditionRequired => 'Die Versionsprüfung fehlt';

  @override
  String get problemRateLimited =>
      'Zu viele Anfragen. Bitte warte einen Moment';

  @override
  String get problemInternalError =>
      'Auf dem Server ist ein Fehler aufgetreten';

  @override
  String get problemUnavailable => 'Der Server ist gerade nicht verfügbar';

  @override
  String get problemMaintenance =>
      'Wartung: Änderungen pausieren, Lesen geht weiter';

  @override
  String get problemSetupRequired => 'Die Instanz ist noch nicht eingerichtet';

  @override
  String get problemForbiddenCapabilityMissing =>
      'Dafür fehlt dir ein Zusatzrecht';

  @override
  String get problemForbiddenLevelInsufficient =>
      'Deine Stufe reicht dafür nicht aus';

  @override
  String get problemForbiddenOwnerOnly => 'Das kann nur ein Owner';

  @override
  String get problemForbiddenViewAsReadOnly =>
      'Nur Lesen, solange du als andere Person ansiehst';

  @override
  String get problemChannelReadOnlyChannelArchived =>
      'Dieser Kanal ist archiviert';

  @override
  String get problemAgentInvocationDeniedProjectNotReady =>
      'Das Projekt des Agenten ist nicht bereit';

  @override
  String problemFallback(String code) {
    return 'Etwas ist schiefgegangen ($code)';
  }

  @override
  String get vizMetricMessagesCreated => 'Nachrichten';

  @override
  String get vizMetricThreadsCreated => 'Neue Threads';

  @override
  String get vizMetricRunsStarted => 'Gestartete Läufe';

  @override
  String get vizMetricRunsFinished => 'Beendete Läufe';

  @override
  String get vizMetricToolCalls => 'Werkzeugaufrufe';

  @override
  String get vizMetricApprovalCards => 'Freigabekarten';

  @override
  String get vizMetricCost => 'Kosten';

  @override
  String get vizMetricInvocationsDenied => 'Abgelehnte Aufrufe';

  @override
  String get vizMetricBillingModeSwitches => 'Wechsel der Abrechnungsart';

  @override
  String get vizMetricCatchups => 'Nachholvorgänge';

  @override
  String get vizMetricResyncs => 'Neusynchronisierungen';

  @override
  String get vizMetricSlowConsumerCloses => 'Trennungen langsamer Clients';

  @override
  String vizMetricUnknown(String code) {
    return 'Kennzahl $code';
  }

  @override
  String get vizSliApiAvailability => 'API-Verfügbarkeit';

  @override
  String get vizSliDeliveryLatency => 'Zustell-Latenz';

  @override
  String get vizSliReconnect => 'Wiederverbindung';

  @override
  String get vizSliPushAcceptance => 'Push-Annahme';

  @override
  String get vizSliRunStart => 'Lauf-Start';

  @override
  String get vizSliTurnDuration => 'Turn-Dauer';

  @override
  String get vizSloOnTarget => 'im Ziel';

  @override
  String get vizSloBudgetTight => 'Budget knapp';

  @override
  String get vizSloTargetMissed => 'Ziel verfehlt';

  @override
  String get vizSloNotEnoughData => 'zu wenige Daten';

  @override
  String get vizOwnMeasurement => 'Eigene Messung';

  @override
  String vizTrafficLightLabel(String slo, String state) {
    return '$slo: $state, eigene Messung';
  }

  @override
  String vizTrafficLightChanged(String slo, String state) {
    return '$slo ist jetzt $state';
  }

  @override
  String get vizCostEstimate => 'Schätzung';

  @override
  String get vizCostInformationOnly => 'nur zur Information';

  @override
  String get vizBadgePersonal => 'Persönlich';

  @override
  String get vizBadgeOrganization => 'Organisation';

  @override
  String get vizApprovalWaiting => 'Freigabe wartet';

  @override
  String get vizMoreRunsThanShown => 'Mehr Läufe als angezeigt';

  @override
  String get vizRunStateWaitingInput => 'Wartet auf Eingabe';

  @override
  String get vizRunStateUnknown => 'Unbekannter Zustand';

  @override
  String vizStepToolCall(String name) {
    return 'Werkzeug $name';
  }

  @override
  String vizStepStatus(String name) {
    return 'Status $name';
  }

  @override
  String vizStepSubagent(String name) {
    return 'Subagent $name';
  }

  @override
  String vizStepWorkflow(String name) {
    return 'Workflow $name';
  }

  @override
  String vizStepBackgroundJob(String name) {
    return 'Hintergrundjob $name';
  }

  @override
  String vizLaneLabel(String space, String channel, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count aktive Läufe',
      one: '1 aktiver Lauf',
      zero: 'keine aktiven Läufe',
    );
    return '$space, $channel, $_temp0';
  }

  @override
  String vizBarRowLabel(String row, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Läufe',
      one: '1 Lauf',
    );
    return '$row, $_temp0';
  }

  @override
  String get vizChartRates => 'Raten';

  @override
  String get vizChartRuns => 'Läufe';

  @override
  String vizTileUp(String delta) {
    return '$delta mehr als im Vorzeitraum';
  }

  @override
  String vizTileDown(String delta) {
    return '$delta weniger als im Vorzeitraum';
  }

  @override
  String get vizTileSame => 'wie im Vorzeitraum';

  @override
  String get vizNotShown => 'nicht angezeigt';

  @override
  String get vizStale => 'Keine neuen Daten';

  @override
  String get vizHarnessTitle => 'Testumgebung der Visualisierungen';

  @override
  String get vizHarnessTheme => 'Design';

  @override
  String get vizHarnessThemeLight => 'Hell';

  @override
  String get vizHarnessThemeDark => 'Dunkel';

  @override
  String get vizHarnessDensity => 'Dichte';

  @override
  String get vizHarnessDensityComfortable => 'Komfortabel';

  @override
  String get vizHarnessDensityCompact => 'Kompakt';

  @override
  String get vizHarnessDensityTouch => 'Touch';

  @override
  String get vizHarnessTextScale => 'Textgröße';

  @override
  String get vizHarnessLanguage => 'Sprache';

  @override
  String get vizHarnessFixtureState => 'Fixture-Zustand';

  @override
  String get vizHarnessTimeLapse => 'Zeitraffer';

  @override
  String get vizMarkLane => 'Bahn';

  @override
  String get vizMarkRunCard => 'Laufkarte';

  @override
  String get vizMarkArea => 'Fläche';

  @override
  String get vizMarkBar => 'Balken';

  @override
  String get vizMarkTrafficLight => 'Ampel';

  @override
  String get vizMarkMetricTile => 'Kennzahl-Kachel';

  @override
  String get messageShowMore => 'Mehr anzeigen';

  @override
  String get messageEdited => '(bearbeitet)';

  @override
  String get messageDeleted => 'Nachricht gelöscht';

  @override
  String get messageResend => 'Erneut senden';

  @override
  String get messageEdit => 'Bearbeiten';

  @override
  String get messageDiscard => 'Verwerfen';

  @override
  String get messageActionReply => 'Antworten';

  @override
  String get messageActionReact => 'Reagieren';

  @override
  String get messageActionPin => 'Anheften';

  @override
  String get messageActionMore => 'Mehr';

  @override
  String get messageMentionsYou => 'Erwähnt dich';

  @override
  String messageSemanticsLabel(String name, String time, String text) {
    return '$name, $time, $text';
  }

  @override
  String get messageOriginAgent => 'AGENT';

  @override
  String get messageOriginApp => 'APP';

  @override
  String get messageOriginImport => 'IMPORT';

  @override
  String get messageOriginSystem => 'SYSTEM';

  @override
  String messageReplyTo(String name) {
    return 'Antwort auf $name';
  }

  @override
  String messageAttachment(String name) {
    return 'Anhang $name';
  }

  @override
  String messageReaction(int count, String emoji) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Reaktionen $emoji',
      one: '1 Reaktion $emoji',
    );
    return '$_temp0';
  }

  @override
  String get messageVoicePlay => 'Sprachnachricht abspielen';

  @override
  String get messageVoicePause => 'Sprachnachricht anhalten';

  @override
  String get messageVoiceTranscriptPending => 'Transkript folgt';

  @override
  String messageSystemThreadReopened(String name) {
    return 'Wieder geöffnet durch Nachricht von $name';
  }

  @override
  String get messageSystemResumeFailed =>
      'Die Agenten-Sitzung konnte nicht fortgesetzt werden';

  @override
  String messageSystemUnknown(String code) {
    return 'Systemnachricht ($code)';
  }

  @override
  String get triggerStateCancelled => 'Abgebrochen';

  @override
  String triggerStateCancelledWithReason(String reason) {
    return 'Abgebrochen: $reason';
  }

  @override
  String triggerStateUnknown(String code) {
    return 'Zustand $code';
  }

  @override
  String liveBoxStateUnknown(String code) {
    return 'Zustand $code';
  }

  @override
  String liveBoxStopRun(String agent) {
    return 'Lauf von $agent stoppen';
  }

  @override
  String liveBoxSemanticsLabel(
    String agent,
    String state,
    String duration,
    int percent,
  ) {
    return 'Lauf von $agent: $state, seit $duration, Kontext $percent Prozent';
  }

  @override
  String get liveBoxFollowupHint =>
      'Nachrichten in diesem Thread erreichen den Agenten als Nachtrag';

  @override
  String liveBoxTokens(String count) {
    return '$count Tokens';
  }

  @override
  String liveBoxContext(String percent) {
    return 'Kontext $percent';
  }

  @override
  String liveBoxRan(String duration) {
    return 'Lief $duration';
  }

  @override
  String liveBoxTools(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Werkzeuge',
      one: '1 Werkzeug',
    );
    return '$_temp0';
  }

  @override
  String liveBoxSubagents(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Unteragenten',
      one: '1 Unteragent',
    );
    return '$_temp0';
  }

  @override
  String get liveBoxExpand => 'Laufdetails anzeigen';

  @override
  String get liveBoxCollapse => 'Laufdetails ausblenden';

  @override
  String get liveBoxOpen => 'Live-Box öffnen';

  @override
  String liveBoxAnnounceQuestion(String agent) {
    return 'Frage von $agent';
  }

  @override
  String liveBoxAnnounceState(String agent, String state) {
    return 'Lauf von $agent: $state';
  }

  @override
  String get toolChipWaiting => 'wartet';

  @override
  String toolChipRunning(String duration) {
    return 'läuft $duration';
  }

  @override
  String get toolChipFailed => 'fehlgeschlagen';

  @override
  String get toolChipCancelled => 'abgebrochen';

  @override
  String get toolChipCompleted => 'abgeschlossen';

  @override
  String toolChipUnknownStatus(String code) {
    return 'Status $code';
  }

  @override
  String get toolChipKindToolCall => 'Werkzeug';

  @override
  String get toolChipKindStatus => 'Status';

  @override
  String get toolChipKindSubagent => 'Unteragent';

  @override
  String get toolChipKindWorkflow => 'Workflow';

  @override
  String get toolChipKindBackgroundJob => 'Hintergrundjob';

  @override
  String get toolChipKindUnknown => 'Schritt';

  @override
  String toolChipLabel(String kind, String name, String status) {
    return '$kind $name, $status';
  }

  @override
  String get toolChipDetails => 'Details anzeigen';

  @override
  String get threadRowWaitingForYou => 'Wartet auf dich';

  @override
  String get threadRowStateOpen => 'Offen';

  @override
  String get threadRowStateDone => 'Erledigt';

  @override
  String threadRowUnread(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count ungelesen',
      one: '1 ungelesen',
      zero: 'nichts ungelesen',
    );
    return '$_temp0';
  }

  @override
  String threadRowMentions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Erwähnungen',
      one: '1 Erwähnung',
    );
    return '$_temp0';
  }

  @override
  String threadRowLabel(
    String title,
    String state,
    String time,
    String unread,
  ) {
    return '$title, $state, $time, $unread';
  }

  @override
  String threadRowOperator(String name) {
    return 'Bediener $name';
  }

  @override
  String get channelRowRunActive => 'Lauf aktiv';

  @override
  String get channelRowMuted => 'stummgeschaltet';

  @override
  String get spaceRowUnread => 'Ungelesen';

  @override
  String spaceRowExpand(String name) {
    return '$name ausklappen';
  }

  @override
  String spaceRowCollapse(String name) {
    return '$name einklappen';
  }

  @override
  String componentsTextFrom(String agent) {
    return 'Text von $agent';
  }

  @override
  String componentsHeaderRequest(String agent) {
    return 'Anfrage von $agent';
  }

  @override
  String componentsHeaderRun(String run) {
    return 'Lauf $run';
  }

  @override
  String componentsHeaderCapability(String capability) {
    return 'Erfordert das Recht \'$capability\'';
  }

  @override
  String componentsApproverRunStarter(String name) {
    return 'Nur $name oder ein Owner kann das freigeben';
  }

  @override
  String get componentsApproverOwner => 'Nur ein Owner kann das freigeben';

  @override
  String get componentsApproverAny =>
      'Alle mit Zugang zu diesem Kanal können antworten';

  @override
  String componentsDeniedCapability(String capability) {
    return 'Dafür fehlt dir das Recht \'$capability\'';
  }

  @override
  String get capabilityApproveActions => 'Aktionen freigeben';

  @override
  String componentsUsed(String name, String choice, String time) {
    return '$name hat \'$choice\' gewählt, $time';
  }

  @override
  String get componentsExpired => 'Abgelaufen';

  @override
  String get componentsDisabled => 'Nicht mehr verfügbar';

  @override
  String get componentsSending => 'Deine Wahl wird gesendet';

  @override
  String get componentsSubmit => 'Absenden';

  @override
  String get componentsSelectPlaceholder => 'Auswählen';

  @override
  String get componentsFieldRequired => 'Pflichtfeld';

  @override
  String componentsFieldTooLong(int max) {
    return 'Höchstens $max Zeichen';
  }

  @override
  String get componentsFieldNumber => 'Zahl eingeben';

  @override
  String get componentsUnsupported => 'Dieses Element braucht eine neuere App';

  @override
  String composerMessageTo(String target) {
    return 'Nachricht an $target';
  }

  @override
  String composerFollowupTo(String agent) {
    return 'Nachtrag an $agent';
  }

  @override
  String composerTyping(String name) {
    return '$name schreibt gerade';
  }

  @override
  String get composerTypingSeveral => 'Mehrere schreiben gerade';

  @override
  String get composerSend => 'Senden';

  @override
  String get composerAttach => 'Datei anhängen';

  @override
  String get composerVoice => 'Sprachnachricht aufnehmen';

  @override
  String composerOptionMissing(String option) {
    return 'Fehlende Option: $option';
  }

  @override
  String get composerOffline =>
      'Offline: deine Nachricht wartet in der Warteschlange';

  @override
  String get composerReadOnly => 'Nur lesen in diesem Kanal';

  @override
  String composerSuggestions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Vorschläge',
      one: '1 Vorschlag',
      zero: 'Keine Vorschläge',
    );
    return '$_temp0';
  }

  @override
  String get composerControlCommand => 'Steuerbefehl';

  @override
  String get billingPersonal => 'Persönlich';

  @override
  String get billingOrganization => 'Organisation';

  @override
  String billingUnknown(String code) {
    return 'Backend $code';
  }

  @override
  String billingReported(String mode) {
    return 'Ist: $mode';
  }

  @override
  String billingLabelPersonal(String name) {
    return 'Backend: Persönlich, läuft auf dem Abo von $name';
  }

  @override
  String get billingLabelOrganization => 'Backend: Organisation';

  @override
  String get costEstimate => 'Schätzung';

  @override
  String get costInformationOnly => 'nur Information';

  @override
  String markdownImage(String alt, String domain) {
    return 'Bild: $alt ($domain)';
  }

  @override
  String markdownImageNoDomain(String alt) {
    return 'Bild: $alt';
  }

  @override
  String get markdownCopyCode => 'Code kopieren';

  @override
  String get triggerReasonMultipleAgentsMentioned =>
      'Bitte nur einen Agenten erwähnen.';

  @override
  String get triggerReasonAgentNotAllowedHere =>
      'Dieser Agent antwortet hier nicht.';

  @override
  String get triggerReasonNoTriggerRight =>
      'Du darfst hier keine Agenten auslösen.';

  @override
  String get triggerReasonRunActiveOtherAgent =>
      'Ein anderer Agent arbeitet gerade in diesem Thread.';

  @override
  String get triggerReasonAgentDisabled => 'Agent ist abgeschaltet.';

  @override
  String get triggerReasonBillingModePersonalNotHolder =>
      'Dieser Agent läuft auf einem persönlichen Abo.';

  @override
  String get triggerReasonVoiceUnsupported =>
      'Sprachnachrichten werden hier nicht verstanden.';

  @override
  String get triggerReasonRunActive =>
      'Der Agent arbeitet gerade; Befehl nach dem Lauf erneut senden.';

  @override
  String get liveBoxReasonUserCancelledNoName => 'Von einer Person abgebrochen';

  @override
  String get threadRowStateWaiting => 'Wartet auf Antwort';

  @override
  String get vizViewLiveTopology => 'Live-Topologie';

  @override
  String get vizViewBackend => 'Backend-Ansicht';

  @override
  String get vizViewDelivery => 'Zustellung und Sync live';

  @override
  String get vizViewExecutive => 'Executive-Seite';

  @override
  String get vizViewTimeline => 'Tages-Timeline';

  @override
  String get vizViewTv => 'TV-Modus';

  @override
  String get vizAsTable => 'Als Tabelle';

  @override
  String get vizOpenGrafana => 'In Grafana öffnen';

  @override
  String get vizLoading => 'Wird geladen';

  @override
  String get vizOnlyForOwner => 'Nur für den Eigentümer';

  @override
  String vizSortBy(String column) {
    return 'Sortieren nach $column';
  }

  @override
  String vizFrameTime(String time) {
    return 'Stand $time';
  }

  @override
  String get vizNoActiveRuns => 'Keine aktiven Läufe';

  @override
  String get vizRatesLastMinute => 'Letzte Minute';

  @override
  String get vizNone => 'keine';

  @override
  String get vizColSpace => 'Bereich';

  @override
  String get vizColChannel => 'Kanal';

  @override
  String get vizColAgent => 'Agent';

  @override
  String get vizColState => 'Zustand';

  @override
  String get vizColStep => 'Aktueller Schritt';

  @override
  String get vizColMode => 'Abrechnungsart';

  @override
  String get vizColApproval => 'Bestätigung';

  @override
  String get vizColCost => 'Bisherige Kosten';

  @override
  String get vizColMetric => 'Kennzahl';

  @override
  String get vizColLastMinute => 'Letzte Minute';

  @override
  String get vizColValue => 'Wert';

  @override
  String get vizColNote => 'Hinweis';

  @override
  String get vizColPresence => 'Anwesenheit';

  @override
  String get vizColConfiguredMode => 'Eingestellte Abrechnungsart';

  @override
  String get vizColReportedMode => 'Gemeldete Abrechnungsart';

  @override
  String get vizColSupportedModes => 'Unterstützte Abrechnungsarten';

  @override
  String get vizColSwitches => 'Wechsel der Abrechnungsart';

  @override
  String get vizColI2Denied => 'Vor der Zustellung abgelehnt (I2)';

  @override
  String get vizColI3Denied =>
      'An abweichender Abrechnungsart gescheitert (I3)';

  @override
  String get vizColCosts => 'Kosten je Abrechnungsart';

  @override
  String get vizColRateLimit => 'Ratenlimit';

  @override
  String get vizPresenceOnline => 'Online';

  @override
  String get vizPresenceDegraded => 'Eingeschränkt';

  @override
  String get vizPresenceOffline => 'Offline';

  @override
  String get vizPresenceUnknown => 'Unbekannt';

  @override
  String get vizFollowsRun => 'je Lauf';

  @override
  String vizReportedMismatch(String mode) {
    return 'Gemeldet: $mode';
  }

  @override
  String get vizRateLimitOk => 'im Rahmen';

  @override
  String get vizRateLimitNearLimit => 'nahe am Limit';

  @override
  String get vizRateLimitLimited => 'begrenzt';

  @override
  String get vizRateLimitUnknown => 'unbekannt';

  @override
  String vizRateLimitResets(String time) {
    return 'wieder frei um $time';
  }

  @override
  String get vizSliNoEvents => 'keine Ereignisse';

  @override
  String vizSliValue(int count, String share) {
    final intl.NumberFormat countNumberFormat =
        intl.NumberFormat.decimalPattern(localeName);
    final String countString = countNumberFormat.format(count);

    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$countString Ereignisse',
      one: '1 Ereignis',
    );
    return '$_temp0, $share innerhalb der Schwelle';
  }

  @override
  String vizCatchupValue(String count, String millis) {
    return '$count, zusammen $millis ms';
  }

  @override
  String get vizConnectionsUser => 'Verbindungen von Personen';

  @override
  String get vizConnectionsAgent => 'Verbindungen von Agenten';

  @override
  String get vizConnectionsApp => 'Verbindungen von Apps';

  @override
  String get vizChartHistory => 'Verlauf';

  @override
  String get vizTileUsage => 'Nutzung';

  @override
  String get vizTileRuns => 'Läufe und Fehlerquote';

  @override
  String get vizTileCosts => 'Kosten je Abrechnungsart';

  @override
  String get vizTileSlo => 'SLO-Ampel';

  @override
  String get vizColSlo => 'SLO';

  @override
  String get vizOwnMeasurementNote =>
      'Werte können von Prometheus abweichen: die eigene Messung zählt im Prozess und kann bei einem Absturz ein Schreibintervall verlieren.';

  @override
  String get vizColRun => 'Agent und Kanal';

  @override
  String get vizColStart => 'Beginn';

  @override
  String get vizColEnd => 'Ende';

  @override
  String vizPlayhead(String time) {
    return 'Abspielposition $time';
  }

  @override
  String get vizPlay => 'Abspielen';

  @override
  String get vizPause => 'Anhalten';

  @override
  String vizSpeed(int speed) {
    return '${speed}x';
  }

  @override
  String get vizJump => 'Zu einer Minute springen';

  @override
  String vizTvPosition(String title, int index, int count) {
    return '$title ($index von $count)';
  }

  @override
  String get consoleNavLive => 'Live';

  @override
  String get consoleNavStats => 'Statistik';

  @override
  String get consoleNavHosts => 'Rechner und Terminal';

  @override
  String get consoleNavPeople => 'Personen';

  @override
  String get consoleNavGroups => 'Gruppen und Freigaben';

  @override
  String get consoleNavDevices => 'Geräte';

  @override
  String get consoleNavAgents => 'Agenten und Kosten';

  @override
  String get consoleNavApps => 'Apps und Webhooks';

  @override
  String get consoleNavAudit => 'Audit';

  @override
  String get consoleNavImport => 'Import';

  @override
  String get consoleNavInstance => 'Instanz und Branding';

  @override
  String get consoleNavStatus => 'Status';

  @override
  String get consoleNavigationLabel => 'Navigation der Konsole';

  @override
  String consoleTitle(String product) {
    return '$product Konsole';
  }

  @override
  String get consolePlaceholderPage =>
      'Diese Seite kommt mit einer späteren Version der Konsole.';

  @override
  String consoleSignedInAs(String name) {
    return 'Angemeldet: $name';
  }

  @override
  String get consoleLogout => 'Abmelden';

  @override
  String consoleThreadInApp(String id) {
    return 'Thread $id: Im Browser gibt es keinen Chat. Öffne ihn in der App.';
  }

  @override
  String get consoleCopyThreadId => 'Thread-ID kopieren';

  @override
  String get consoleCopied => 'Kopiert';

  @override
  String get consoleWorking => 'Einen Moment';

  @override
  String consoleLoginTitle(String product) {
    return 'Bei $product anmelden';
  }

  @override
  String consoleLoginWithProvider(String provider) {
    return 'Mit $provider anmelden';
  }

  @override
  String get consoleLoginOr => 'oder';

  @override
  String get consoleLoginName => 'Anmeldename';

  @override
  String get consoleLoginWithPasskey => 'Mit Passkey anmelden';

  @override
  String get consoleLoginPasswordAndCode => 'Passwort und Einmal-Code';

  @override
  String get consoleLoginPassword => 'Passwort';

  @override
  String get consoleLoginOneTimeCode => 'Einmal-Code';

  @override
  String get consoleLoginSubmit => 'Anmelden';

  @override
  String get consoleLoginNoMethod =>
      'Für diese Instanz ist keine Anmeldung eingerichtet. Bitte den Eigentümer, eine einzurichten.';

  @override
  String get consoleLoginFailed =>
      'Die Anmeldung ist fehlgeschlagen. Prüfe Anmeldename, Passwort und Einmal-Code und versuche es erneut.';

  @override
  String get consoleLoginPasskeyFailed =>
      'Der Passkey wurde nicht angenommen. Versuche es erneut oder wähle einen anderen Weg.';

  @override
  String get consoleLoginOidcFailed =>
      'Der Anmeldedienst hat die Anmeldung nicht bestätigt. Starte die Anmeldung erneut.';

  @override
  String get consoleLoginOidcCancelled =>
      'Die Anmeldung wurde beim Anmeldedienst abgebrochen. Starte sie erneut oder wähle einen anderen Weg.';

  @override
  String get consoleLoginOidcExpired =>
      'Die Anmeldung hat zu lange gedauert. Starte sie erneut.';

  @override
  String get consoleLoginUnavailable =>
      'Der Server ist nicht erreichbar. Prüfe die Verbindung und versuche es erneut.';

  @override
  String get consoleCallbackWorking => 'Die Anmeldung wird abgeschlossen';

  @override
  String get consoleStepUpTitle => 'Bestätige, dass du es bist';

  @override
  String get consoleStepUpExplanation =>
      'Vor jeder Verwaltung verlangt die Konsole eine frische Bestätigung.';

  @override
  String get consoleStepUpPasskey => 'Mit Passkey bestätigen';

  @override
  String consoleStepUpOidc(String provider) {
    return 'Erneut mit $provider anmelden';
  }

  @override
  String get consoleStepUpPassword => 'Mit Passwort und Einmal-Code bestätigen';

  @override
  String get consoleStepUpConfirm => 'Bestätigen';

  @override
  String get consoleStepUpFailed =>
      'Die Bestätigung ist fehlgeschlagen. Versuche es erneut oder wähle einen anderen Weg.';

  @override
  String get consoleStepUpNoMethod =>
      'Für dich ist kein Weg zur Bestätigung eingerichtet. Bitte den Eigentümer, dir einen Passkey einzurichten.';

  @override
  String get consoleLoggedOutTitle => 'Du bist abgemeldet.';

  @override
  String consoleLoggedOutReason(String reason) {
    return 'Grund: $reason';
  }

  @override
  String get consoleLoggedOutByYou =>
      'Du hast dich von der Konsole abgemeldet.';

  @override
  String get consoleLogInAgain => 'Erneut anmelden';

  @override
  String get consoleUpdatedReload =>
      'Die Konsole wurde aktualisiert. Lade die Seite neu, um weiterzumachen.';

  @override
  String get consoleReload => 'Neu laden';
}
