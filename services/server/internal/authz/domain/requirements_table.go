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
		"auth.login.begin", "auth.login.complete", "instance.discovery.read", "status.public.read",
	}},
	{"self", Self, []string{
		"auth.logout", "auth.step_up", "draft.read", "identity.link", "local_credential.set",
		"mention.list", "passkey.register", "push.register", "push.unregister", "quiet_hours.update",
		"self.profile.update", "session.extension.create", "settings.read", "settings.update",
	}},
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
		"instance.bootstrap", "owner.recover", "signing_key.transition", "terminal.terminate",
	}},
	{"self oder owner", Any(Self, Owner), []string{
		"device.list", "device.revoke", "gdpr.request.create", "gdpr.request.read", "identity.read",
		"identity.unlink", "passkey.revoke", "session.list", "session.revoke",
	}},
	{"participant", Participant, []string{"dm.leave"}},
	{"self und read", All(Self, AtLeast(LevelRead)), []string{
		"draft.write", "notification_pref.update", "read_state.mark_unread", "read_state.update",
		"sidebar.update",
	}},
	{"auth", Auth, []string{
		"events.catch_up", "instance.read", "run.overview.read", "search.query", "status.read",
		"sync.snapshot.read",
	}},
	{"read und +export", All(AtLeast(LevelRead), HasCap(CapExport)), []string{"export.channel"}},
	{"self oder manage", Any(Self, HasCap(CapManage)), []string{"file.delete", "template.manage"}},
	{"read", Deferred("M3"), []string{"file.read"}},
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
	{"host-self", Deferred("M2"), []string{"host.connect"}},
	{"owner", Deferred("M2"), []string{"host.manage"}},
	{"public (gültiger Kopplungs-Code)", Deferred("M2"), []string{"host.pair"}},
	{"owner oder assignee oder +terminal", Deferred("M2"), []string{"host.read"}},
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
	{"agent-self und Marke gültig", Deferred("M2"), []string{"run.start"}},
	{"auth und self", All(Auth, Self), []string{"self.read"}},
	{"owner oder manage", ByKind(map[string]Expr{kindInstance: Owner, kindSpace: Any(Owner, HasCap(CapManage))}), []string{
		"stats.read",
	}},
	{"read bzw. self", ByKind(map[string]Expr{kindChannel: AtLeast(LevelRead), kindUserRes: Self}), []string{
		"template.read",
	}},
	{"self und +terminal", Deferred("M2"), []string{"terminal.attach"}},
	{"self oder owner", Deferred("M2"), []string{"terminal.close", "terminal.read_own"}},
	{"human und +terminal", Deferred("M2"), []string{"terminal.open"}},
	{"self", Deferred("M2"), []string{"terminal.stream"}},
	{"manage auf Quelle und Ziel", All(HasCap(CapManage), TargetHasCap(CapManage)), []string{"thread.move"}},
	{"write", All(AtLeast(LevelWrite), HasCapIf(CapManage, FactRestrictedTags)), []string{"thread.tags.set"}},
	{"write und (Ersteller oder manage oder Bediener-Agent)", All(AtLeast(LevelWrite), Any(Self, HasCap(CapManage), ThreadOperator)), []string{
		"thread.title.update",
	}},
	{"owner; self bei eigenem", Any(Self, Owner), []string{"token.create", "token.revoke"}},
	{"owner; self bei eigenen", Any(Self, Owner), []string{"token.list"}},
	{"human und write", All(Human, AtLeast(LevelWrite)), []string{"typing.start"}},
	{"human; Apps mit Scope", Any(Human, AppKind), []string{"user.read"}},
	{"public mit Token; App braucht write", Deferred("M2"), []string{"webhook_in.post"}},
	{"Freigaben und Scope des Empfängers", Deferred("M2"), []string{"webhook_out.deliver"}},
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
