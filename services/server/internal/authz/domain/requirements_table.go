// SPDX-License-Identifier: Apache-2.0

package domain

// requirementGroups is the requirement table, written by hand from the column "Recht" of
// docs/spec/05-aktionen.md with the mapping of the brief M1-28 ("Requirement table"): one row per
// 05 text and expression, with the actions that carry it. Source repeats the 05 text verbatim,
// so each text appears once in this file. Overrides by action id (message.create,
// thread.tags.set, group.read, stats.read, grant.read) and the deferred entries of 07 section 4.2
// are rows of their own and win over the text rule; typing.start ("human und write") reads as
// All(Human, Level(write)) (docs/DECISIONS.md M1-28). A ticket that builds a deferred action
// moves it to the row of its expression and removes it from testdata/deferred_actions.golden.
//
//nolint:goconst,misspell // action ids repeat the generated catalogue, and Source copies the German 05 text ("Autor")
var requirementGroups = []struct {
	source  string
	expr    Expr
	actions []string
}{
	{"owner oder manage", Any(Owner, HasCap(CapManage)), []string{"access.explain", "grant.revoke"}},
	{"Recht der Zielaktion", Preview, []string{"access.preview"}},
	{"owner", Owner, []string{
		"access.view_as", "agent.billing_mode.set", "agent.billing_mode.set_all", "agent.create",
		"agent.credential_holder.set", "agent.disable", "agent.enable", "agent.tool_class.set",
		"agent.update", "app.install", "app.read", "app.resume", "app.scopes.update", "app.suspend",
		"app.uninstall", "audit.read", "audit.verify", "channel.operator.set", "channel.project.bind",
		"cost.reconcile", "dev_environment.manage", "feature_flag.read", "feature_flag.update",
		"gdpr.request.decide", "group.create", "group.delete", "group.member.add", "group.member.remove",
		"group.update", "identity_provider.manage", "identity_provider.read", "import.manage",
		"import.read", "instance.branding.update", "instance.settings.read", "instance.settings.update",
		"owner.add", "owner.remove", "price.manage", "price.read", "signing_key.rotate", "space.archive",
		"space.create", "space.operator.set", "space.reorder", "status.manage", "user.create",
		"user.disable", "user.enable", "user.update", "webhook_out.manage", "webhook_out.read",
		"webhook_out.redeliver",
	}},
	{"agent-self und wirksamer Bediener", ChannelOperator, []string{"agent.config.read"}},
	{"agent-self", AgentSelf, []string{
		"agent.events.consume", "agent.heartbeat", "agent.triggerers.read", "agent_session.report",
		"run.context.read", "run.finish", "run.update",
	}},
	{"trigger und Regeln aus 5", AtLeast(LevelTrigger), []string{"agent.invoke"}},
	{"owner oder agent-self oder read auf einem Kanal, den der Agent bedient", Any(Owner, AgentSelf, ReadOnOperatedChannel), []string{
		"agent.read",
	}},
	{"trigger und `agent.invoke`", Deferred("M2"), []string{"agent_session.bind", "agent_session.reset"}},
	{"read", AtLeast(LevelRead), []string{
		"agent_session.read", "channel.read", "command.read", "cost.read.thread", "gateway.focus",
		"message.read", "message.revisions.read", "push.deliver", "run.read", "space.read",
		"thread.read",
	}},
	{"public", Public, []string{
		"auth.login.begin", "auth.login.complete", "console.csp_report", "instance.discovery.read", "status.public.read",
	}},
	{"self", Self, []string{
		"access.view_as.end", "auth.logout", "auth.step_up", "identity.link", "local_credential.set",
		"mention.list", "passkey.register", "push.register", "push.unregister",
		"self.profile.update", "session.extension.create",
	}},
	// The own read states, drafts and settings (M1-61 note 22): data of a person (03 read_states,
	// drafts, user_settings reference users), so an agent or app naming itself gets
	// subject_kind_not_allowed from authorize, never from the module.
	// The quiet hours of M3-18 are a person's data the same way (03 quiet_hours references users).
	{"self", All(Human, Self), []string{"draft.read", "quiet_hours.update", "settings.read", "settings.update"}},
	{"public (gültiges Refresh-Token und DPoP-Beweis der Familie)", Public, []string{"auth.token.refresh"}},
	{"manage", HasCap(CapManage), []string{
		"channel.archive", "channel.behavior.read", "channel.behavior.update", "channel.create",
		"channel.read_only.set", "channel.reorder", "channel.update", "forum_tag.manage", "pin.create",
		"pin.delete", "space.update", "webhook_in.create", "webhook_in.read", "webhook_in.revoke",
	}},
	{"human", Human, []string{"client_metrics.submit", "dm.create"}},
	{"trigger und `agent.invoke` ohne Lauf (bei App-Befehlen `required_level`)", Deferred("M2"), []string{
		"command.autocomplete",
	}},
	{"agent-self bzw. app-self, Eigentümer des Befehls", Any(AgentSelf, AppSelf), []string{
		"command.autocomplete.respond",
	}},
	{"`required_level` des Befehls und Regeln wie beschrieben", Deferred("M2"), []string{"command.invoke"}},
	{"agent-self bzw. app-self mit Scope", Any(AgentSelf, AppSelf), []string{
		"command.register", "command.remove",
	}},
	{"Autor der Nachricht (agent-self bzw. app-self)", Any(AgentSelf, AppSelf), []string{"component.update"}},
	{"owner; Bereich mit manage", ByKind(map[string]Expr{kindInstance: Owner, kindAgentRes: Owner, kindSpace: Any(Owner, HasCap(CapManage))}), []string{
		"cost.read.aggregate",
	}},
	{"App mit Scope `costs:reconcile`", AppKind, []string{"cost.reconcile.submit"}},
	{"owner oder self", Any(Self, Owner), []string{"dev_environment.read", "device.approve"}},
	{"agent-self (Runner der Umgebung)", AgentSelf, []string{"dev_environment.report"}},
	{"system", System, []string{
		"device.expire", "grant.expire", "group.member.expire", "group.member.sync",
		"instance.bootstrap", "owner.recover", "signing_key.transition", "terminal.terminate", "webhook_out.transition",
	}},
	{"self oder owner", Any(Self, Owner), []string{
		"device.list", "device.revoke", "gdpr.request.create", "gdpr.request.read", "identity.read",
		"identity.unlink", "passkey.revoke", "session.list", "session.revoke",
	}},
	{"participant", Participant, []string{"dm.leave"}},
	{"self und read", All(Self, AtLeast(LevelRead)), []string{"sidebar.update"}},
	// notification_pref.update is a person's data too (M3-18, 03 notification_preferences
	// references users): agents and apps get subject_kind_not_allowed from authorize.
	{"self und read", All(Human, Self, AtLeast(LevelRead)), []string{
		"draft.write", "notification_pref.update", "read_state.mark_unread", "read_state.update",
	}},
	{"auth", Auth, []string{
		"events.catch_up", "instance.read", "run.overview.read", "search.query", "status.read",
		"sync.snapshot.read",
	}},
	{"read und +export", All(AtLeast(LevelRead), HasCap(CapExport)), []string{"export.channel"}},
	{"self oder manage", Any(Self, HasCap(CapManage)), []string{"file.delete", "template.manage"}},
	// file.read by purpose (05 section 2.2; M3-03, brief M3-10 design note 6): an attachment needs
	// read on its path (the visibility of kind file adds the uploader rule); an avatar or a logo
	// any signed-in principal; gdpr_export read, never reached because its visibility refuses it;
	// an event row without facts read on its path or, without a path, the audience itself.
	{"read", ByFilePurpose(map[string]Expr{
		purposeAttachment: AtLeast(LevelRead), purposeAvatar: Auth, purposeBranding: Auth, purposeGDPRExport: AtLeast(LevelRead),
		FileEventPath: AtLeast(LevelRead), FileEventSelf: Self,
	}), []string{"file.read"}},
	{"write", AtLeast(LevelWrite), []string{
		"file.transcribe", "reaction.add", "thread.close", "thread.create", "thread.reopen",
	}},
	{"agent-self: Bediener des Threads, an dessen Nachricht die Datei hängt, und read", Deferred("M3"), []string{
		"file.transcript.submit",
	}},
	{"write und +upload", All(AtLeast(LevelWrite), HasCap(CapUpload)), []string{"file.upload"}},
	{"auth (Mensch mit aktivem Gerät, Agent, App mit `events:subscribe`)", Auth, []string{"gateway.connect"}},
	{"self (Betroffene Person) oder owner", Any(Self, Owner), []string{"gdpr.export.download"}},
	{"owner oder manage (auf `host` nur owner)", Any(Owner, HasCap(CapManage)), []string{"grant.create"}},
	{"owner oder manage", ByKind(map[string]Expr{kindSpace: Any(Owner, HasCap(CapManage)), kindChannel: Any(Owner, HasCap(CapManage)), kindHost: Owner, kindPrincipal: Owner}), []string{
		"grant.read",
	}},
	{"owner oder self", Any(Owner, GroupMember), []string{"group.read"}},
	// host.connect (M2-43): only the host itself, authenticated with its key (07 section 3).
	{"host-self", HostSelf, []string{"host.connect"}},
	// host.manage and host.read are gated by HumanSession (M2-39, 05 section 4.14).
	{"owner", All(HumanSession, Owner), []string{"host.manage"}},
	// host.pair (M2-41): the use case proves the pairing code before authorize runs, as for the
	// refresh token; the host is visible to any principal for exactly this requirement.
	{"public (gültiger Kopplungs-Code)", Public, []string{"host.pair"}},
	{"owner oder assignee oder +terminal", All(HumanSession, Any(Owner, Assignee, Terminal)), []string{"host.read"}},
	{"owner", Deferred("M8"), []string{"import.apply"}},
	{"trigger", Deferred("M2"), []string{"interaction.create"}},
	{"agent-self bzw. app-self", Any(AgentSelf, AppSelf), []string{"interaction.respond"}},
	{"write", All(AtLeast(LevelWrite), HasCapIf(CapUpload, FactHasAttachments)), []string{"message.create"}},
	{"(write und Autor) oder manage", Any(All(AtLeast(LevelWrite), Self), HasCap(CapManage)), []string{
		"message.delete",
	}},
	{"write und Autor", All(AtLeast(LevelWrite), Self), []string{"message.update"}},
	{"write und self", All(AtLeast(LevelWrite), Self), []string{"reaction.remove"}},
	{"trigger und (Auslöser des Laufs oder manage)", All(AtLeast(LevelTrigger), Any(Self, HasCap(CapManage))), []string{
		"run.cancel",
	}},
	{"write und `agent.invoke`", Deferred("M2"), []string{"run.followup.create"}},
	// The invocation token (M2-09): runs computes the verdict, Token decides on it; a valid token
	// starts a run, a valid or expired one renews it.
	{"agent-self und Marke gültig", All(AgentSelf, Token(InvocationValid)), []string{"run.start"}},
	{"agent-self und Marke aktuell", All(AgentSelf, Token(InvocationValid, InvocationExpired)), []string{"run.token.renew"}},
	{"auth und self", All(Auth, Self), []string{"self.read"}},
	{"owner oder manage", ByKind(map[string]Expr{kindInstance: Owner, kindSpace: Any(Owner, HasCap(CapManage))}), []string{
		"stats.read",
	}},
	{"read bzw. self", ByKind(map[string]Expr{kindChannel: AtLeast(LevelRead), kindUserRes: Self}), []string{
		"template.read",
	}},
	// The terminal actions (M2-40, 05 section 4.14): overrides by action id put each behind the gate
	// of HumanSession; Terminal of terminal.attach is evaluated on the session's host.
	{"self und +terminal", All(HumanSession, Self, Terminal), []string{"terminal.attach"}},
	{"self oder owner", All(HumanSession, Any(Self, Owner)), []string{"terminal.close"}},
	{"self oder owner", All(HumanSession, ByKind(map[string]Expr{kindTerminalSession: Any(Self, Owner), kindHost: Auth})), []string{
		"terminal.read_own",
	}},
	{"human und +terminal", All(HumanSession, Terminal), []string{"terminal.open"}},
	{"self", All(HumanSession, Self), []string{"terminal.stream"}},
	{"manage auf Quelle und Ziel", All(HasCap(CapManage), TargetHasCap(CapManage)), []string{"thread.move"}},
	{"write", All(AtLeast(LevelWrite), HasCapIf(CapManage, FactRestrictedTags)), []string{"thread.tags.set"}},
	{"write und (Ersteller oder manage oder Bediener-Agent)", All(AtLeast(LevelWrite), Any(Self, HasCap(CapManage), ThreadOperator)), []string{
		"thread.title.update",
	}},
	// token.create names the owner first, so a caller without either right is refused with
	// owner_only (brief M1-44: a personal token of an owner has no owner rights, SR5).
	{"owner; self bei eigenem", Any(Owner, Self), []string{"token.create"}},
	{"owner; self bei eigenem", Any(Self, Owner), []string{"token.revoke"}},
	{"owner; self bei eigenen", Any(Self, Owner), []string{"token.list"}},
	{"human und write", All(Human, AtLeast(LevelWrite)), []string{"typing.start"}},
	{"human; Apps mit Scope", Any(Human, AppKind), []string{"user.read"}},
	// The post of an incoming webhook (M2-27 design note 3a): the token stage makes the actor the
	// app of the webhook, which needs write on the webhook's channel. The right is not Public, so an
	// anonymous principal fails closed with unauthenticated before the expression; a person gets
	// subject_kind_not_allowed.
	{"public mit Token; App braucht write", All(AppKind, AtLeast(LevelWrite)), []string{"webhook_in.post"}},
	{"Freigaben und Scope des Empfängers", Delivery, []string{"webhook_out.deliver"}},
}

// requirements indexes requirementGroups by action id.
var requirements = func() map[string]Requirement {
	m := map[string]Requirement{}
	for _, g := range requirementGroups {
		for _, a := range g.actions {
			if _, dup := m[a]; dup {
				panic("authz: action " + a + " has two requirement entries")
			}
			m[a] = Requirement{Source: g.source, Expr: g.expr}
		}
	}
	return m
}()
