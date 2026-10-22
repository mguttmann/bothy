// SPDX-License-Identifier: Apache-2.0

package domain

// requirementGroups is the requirement table, written by hand from the column "Permission" of
// docs/spec/05-actions.md with the mapping of the brief M1-28 ("Requirement table"): one row per
// 05 text and expression, with the actions that carry it. Source repeats the 05 text verbatim,
// so each text appears once in this file. Overrides by action id (message.create,
// thread.tags.set, group.read, stats.read, grant.read) and the deferred entries of 07 section 4.2
// are rows of their own and win over the text rule; typing.start ("human and write") reads as
// All(Human, Level(write)) (docs/DECISIONS.md M1-28). A ticket that builds a deferred action
// moves it to the row of its expression and removes it from testdata/deferred_actions.golden.
//
//nolint:goconst // action ids repeat the generated catalogue
var requirementGroups = []struct {
	source  string
	expr    Expr
	actions []string
}{
	{"owner or manage", Any(Owner, HasCap(CapManage)), []string{"access.explain", "grant.revoke"}},
	{"permission of the target action", Preview, []string{"access.preview"}},
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
	{"agent-self and effective operator", ChannelOperator, []string{"agent.config.read"}},
	{"agent-self", AgentSelf, []string{
		"agent.events.consume", "agent.heartbeat", "agent.triggerers.read", "agent_session.report",
		"run.context.read", "run.finish", "run.update",
	}},
	{"trigger and rules from 5", AtLeast(LevelTrigger), []string{"agent.invoke"}},
	{"owner or agent-self or read on a channel that the agent operates", Any(Owner, AgentSelf, ReadOnOperatedChannel), []string{
		"agent.read",
	}},
	{"trigger and `agent.invoke`", Deferred("M2"), []string{"agent_session.bind", "agent_session.reset"}},
	{"read", AtLeast(LevelRead), []string{
		"agent_session.read", "channel.read", "cost.read.thread", "gateway.focus",
		"message.read", "message.revisions.read", "push.deliver", "run.read", "space.read",
		"thread.read",
	}},
	// command.read by resource kind (M2-16 design note 8 d): read on the channel of a list, and on
	// a single command its visibility rule decides (owner, scope target, offered in a channel).
	{"read", ByKind(map[string]Expr{kindChannel: AtLeast(LevelRead), kindCommand: Auth}), []string{"command.read"}},
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
	{"public (valid refresh token and DPoP proof of the family)", Public, []string{"auth.token.refresh"}},
	{"manage", HasCap(CapManage), []string{
		"channel.archive", "channel.behavior.read", "channel.behavior.update", "channel.create",
		"channel.read_only.set", "channel.reorder", "channel.update", "forum_tag.manage", "pin.create",
		"pin.delete", "space.update", "webhook_in.create", "webhook_in.read", "webhook_in.revoke",
	}},
	{"human", Human, []string{"client_metrics.submit", "dm.create"}},
	// command.autocomplete (M2-17 design note 3): an agent owner needs trigger and the agent.invoke
	// guard without a run, an app owner the required_level of the command.
	{"trigger and `agent.invoke` without a run (for app commands `required_level`)", AutocompleteLevel, []string{
		"command.autocomplete",
	}},
	{"agent-self or app-self, owner of the command", Any(AgentSelf, AppSelf), []string{
		"command.autocomplete.respond",
	}},
	// command.invoke (M2-16): the level of the command, then the guard (app owner internal, model
	// command during an active run run_active); agent commands add agent.invoke in runs.
	{"`required_level` of the command and rules as described", CommandLevel, []string{"command.invoke"}},
	{"agent-self or app-self with scope", Any(AgentSelf, AppSelf), []string{
		"command.register", "command.remove",
	}},
	{"author of the message (agent-self or app-self)", Any(AgentSelf, AppSelf), []string{"component.update"}},
	{"owner; space with manage", ByKind(map[string]Expr{kindInstance: Owner, kindAgentRes: Owner, kindSpace: Any(Owner, HasCap(CapManage))}), []string{
		"cost.read.aggregate",
	}},
	{"app with scope `costs:reconcile`", AppKind, []string{"cost.reconcile.submit"}},
	{"owner or app with scope `costs:reconcile`", Any(Owner, AppKind), []string{"cost.reconcile.read"}},
	{"owner or self", Any(Self, Owner), []string{"dev_environment.read", "device.approve"}},
	{"agent-self (runner of the environment)", AgentSelf, []string{"dev_environment.report"}},
	{"system", System, []string{
		"device.expire", "grant.expire", "group.member.expire", "group.member.sync",
		"instance.bootstrap", "owner.recover", "signing_key.transition", "terminal.terminate",
		"webhook_out.transition",
	}},
	{"self or owner", Any(Self, Owner), []string{
		"device.list", "device.revoke", "gdpr.request.create", "gdpr.request.read", "identity.read",
		"identity.unlink", "passkey.revoke", "session.list", "session.revoke",
	}},
	{"participant", Participant, []string{"dm.leave"}},
	{"self and read", All(Self, AtLeast(LevelRead)), []string{"sidebar.update"}},
	// notification_pref.update is a person's data too (M3-18, 03 notification_preferences
	// references users): agents and apps get subject_kind_not_allowed from authorize.
	{"self and read", All(Human, Self, AtLeast(LevelRead)), []string{
		"draft.write", "notification_pref.update", "read_state.mark_unread", "read_state.update",
	}},
	{"auth", Auth, []string{
		"events.catch_up", "instance.read", "run.overview.read", "search.query", "status.read",
		"sync.snapshot.read",
	}},
	{"read and +export", All(AtLeast(LevelRead), HasCap(CapExport)), []string{"export.channel"}},
	{"self or manage", Any(Self, HasCap(CapManage)), []string{"file.delete", "template.manage"}},
	// file.read (M3-03): read on the path; the visibility of kind file adds the uploader rule.
	{"read", AtLeast(LevelRead), []string{"file.read"}},
	{"write", AtLeast(LevelWrite), []string{
		"file.transcribe", "reaction.add", "thread.close", "thread.create", "thread.reopen",
	}},
	{"agent-self: operator of the thread whose message the file is attached to, and read", Deferred("M3"), []string{
		"file.transcript.submit",
	}},
	{"write and +upload", All(AtLeast(LevelWrite), HasCap(CapUpload)), []string{"file.upload"}},
	{"auth (person with an active device, agent, app with `events:subscribe`)", Auth, []string{"gateway.connect"}},
	{"self (data subject) or owner", Any(Self, Owner), []string{"gdpr.export.download"}},
	{"owner or manage (on `host` only owner)", Any(Owner, HasCap(CapManage)), []string{"grant.create"}},
	{"owner or manage", ByKind(map[string]Expr{kindSpace: Any(Owner, HasCap(CapManage)), kindChannel: Any(Owner, HasCap(CapManage)), kindHost: Owner, kindPrincipal: Owner}), []string{
		"grant.read",
	}},
	{"owner or self", Any(Owner, GroupMember), []string{"group.read"}},
	// host.connect (M2-43): only the host itself, authenticated with its key (07 section 3).
	{"host-self", HostSelf, []string{"host.connect"}},
	// host.manage and host.read are gated by HumanSession (M2-39, 05 section 4.14).
	{"owner", All(HumanSession, Owner), []string{"host.manage"}},
	// host.pair (M2-41): the use case proves the pairing code before authorize runs, as for the
	// refresh token; the host is visible to any principal for exactly this requirement.
	{"public (valid pairing code)", Public, []string{"host.pair"}},
	{"owner or assignee or +terminal", All(HumanSession, Any(Owner, Assignee, Terminal)), []string{"host.read"}},
	{"owner", Deferred("M8"), []string{"import.apply"}},
	// interaction.create (M2-15 design note 8 b): trigger on the path; the guard of agent.invoke with
	// the interaction rules follows in step 9.
	{"trigger", AtLeast(LevelTrigger), []string{"interaction.create"}},
	{"agent-self or app-self", Any(AgentSelf, AppSelf), []string{"interaction.respond"}},
	{"write", All(AtLeast(LevelWrite), HasCapIf(CapUpload, FactHasAttachments)), []string{"message.create"}},
	{"(write and author) or manage", Any(All(AtLeast(LevelWrite), Self), HasCap(CapManage)), []string{
		"message.delete",
	}},
	{"write and author", All(AtLeast(LevelWrite), Self), []string{"message.update"}},
	{"write and self", All(AtLeast(LevelWrite), Self), []string{"reaction.remove"}},
	{"trigger and (triggering person of the run or manage)", All(AtLeast(LevelTrigger), Any(Self, HasCap(CapManage))), []string{
		"run.cancel",
	}},
	{"write and `agent.invoke`", Deferred("M2"), []string{"run.followup.create"}},
	// The invocation token (M2-09): runs computes the verdict, Token decides on it; a valid token
	// starts a run, a valid or expired one renews it.
	{"agent-self and token valid", All(AgentSelf, Token(InvocationValid)), []string{"run.start"}},
	{"agent-self and token current", All(AgentSelf, Token(InvocationValid, InvocationExpired)), []string{"run.token.renew"}},
	{"auth and self", All(Auth, Self), []string{"self.read"}},
	{"owner or manage", ByKind(map[string]Expr{kindInstance: Owner, kindSpace: Any(Owner, HasCap(CapManage))}), []string{
		"access.hints.read", "stats.read",
	}},
	{"read or self", ByKind(map[string]Expr{kindChannel: AtLeast(LevelRead), kindUserRes: Self}), []string{
		"template.read",
	}},
	// The terminal actions (M2-40, 05 section 4.14): overrides by action id put each behind the gate
	// of HumanSession; Terminal of terminal.attach is evaluated on the session's host.
	{"self and +terminal", All(HumanSession, Self, Terminal), []string{"terminal.attach"}},
	{"self or owner", All(HumanSession, Any(Self, Owner)), []string{"terminal.close"}},
	{"self or owner", All(HumanSession, ByKind(map[string]Expr{kindTerminalSession: Any(Self, Owner), kindHost: Auth})), []string{
		"terminal.read_own",
	}},
	{"human and +terminal", All(HumanSession, Terminal), []string{"terminal.open"}},
	{"self", All(HumanSession, Self), []string{"terminal.stream"}},
	{"manage on source and target", All(HasCap(CapManage), TargetHasCap(CapManage)), []string{"thread.move"}},
	{"write", All(AtLeast(LevelWrite), HasCapIf(CapManage, FactRestrictedTags)), []string{"thread.tags.set"}},
	{"write and (creator or manage or operator agent)", All(AtLeast(LevelWrite), Any(Self, HasCap(CapManage), ThreadOperator)), []string{
		"thread.title.update",
	}},
	// token.create names the owner first, so a caller without either right is refused with
	// owner_only (brief M1-44: a personal token of an owner has no owner rights, SR5).
	{"owner; self for own token", Any(Owner, Self), []string{"token.create"}},
	{"owner; self for own token", Any(Self, Owner), []string{"token.revoke"}},
	{"owner; self for own tokens", Any(Self, Owner), []string{"token.list"}},
	{"human and write", All(Human, AtLeast(LevelWrite)), []string{"typing.start"}},
	{"human; apps with scope", Any(Human, AppKind), []string{"user.read"}},
	// The post of an incoming webhook (M2-27 design note 3a): the token stage makes the actor the
	// app of the webhook, which needs write on the webhook's channel. The right is not Public, so an
	// anonymous principal fails closed with unauthenticated before the expression; a person gets
	// subject_kind_not_allowed.
	{"public with token; the app needs write", All(AppKind, AtLeast(LevelWrite)), []string{"webhook_in.post"}},
	{"grants and scope of the receiver", Delivery, []string{"webhook_out.deliver"}},
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
