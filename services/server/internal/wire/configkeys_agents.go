// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// agentsConfigKeys returns the keys of the agent consumers from docs/spec/04-sync-protocol.md
// "Configuration values of this chapter" (M2-07): the ack timeout, the save interval of acked_seq
// and the inflight bound per consumer; the keys of the presence of agents (M2-20); and the grace of
// a cancel request (M2-09); and the longest external session reference (M2-12).
func agentsConfigKeys() []config.Key {
	return []config.Key{
		{Name: "agent.ack_timeout", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Without an ack within this time the server delivers again from acked_seq + 1 with redelivered."},
		{Name: "agent.ack_persist_interval", Kind: config.KindDuration, Default: "1s", Min: minDeadline, Chapter: chapter04,
			Doc: "acked_seq of an agent consumer is saved at the latest after this interval."},
		{Name: "agent.max_inflight_events", Kind: config.KindInt, Default: "100", Min: "1", Max: "2147483647", Chapter: chapter04,
			Doc: "Most unacknowledged events on the way per agent consumer."},
		// The presence of agents (M2-20): 04, 03 and 10 "Configuration values of this chapter".
		{Name: keyPresenceTimeout, Kind: config.KindDuration, Default: "90s", Min: minDeadline, Chapter: chapter04,
			Doc: "Without a heartbeat for this long an agent is offline."},
		{Name: "agent.heartbeat_persist_interval", Kind: config.KindDuration, Default: denyAggregateWindowDefault, Min: minDeadline, Chapter: chapter03,
			Doc: "Heartbeat fields are stored at most this often."},
		{Name: "billing.mismatch_alert_after", Kind: config.KindDuration, Default: "5m", Min: minDeadline, Chapter: chapter10,
			Doc: "How long the reported mode may differ from the configured one before the warning and the alarm."},
		// M2-09: a cancel request the adapter does not confirm in time ends the run (09 8).
		{Name: "agent.cancel_grace", Kind: config.KindDuration, Default: thirtySeconds, Min: minDeadline, Chapter: chapter04,
			Doc: "Time for the adapter to report a cancellation; then the server cancels the run with cancel_timeout."},
		// M2-12: external_session_ref of bind, report and run.start (09 9.4, 9.5, 21). The maximum keeps
		// an entry of the unique index on (agent_id, external_session_ref) within the PostgreSQL
		// B-tree entry limit of 8 kB pages for every configured value, so an over-long reference is
		// a field error (too_long), never a database error and a 500 (docs/DECISIONS.md D-M2-12-9).
		{Name: "agent.session_ref_max_bytes", Kind: config.KindInt, Default: "1024", Min: "1", Max: "2048", Chapter: chapter09,
			Doc: "Longest external_session_ref and model of an agent session in bytes; the maximum keeps an entry of the unique index on (agent_id, external_session_ref) within the PostgreSQL B-tree entry limit."},
		// A1-03: the lifetime of a token of a built-in agent (26 3.3); at most
		// auth.api_token_max_ttl (cross-check hosted_token_ttl_within_api_token_max_ttl).
		{Name: keyHostedTokenTTL, Kind: config.KindDuration, Default: defaultDay, Min: "1s", Chapter: chapter26,
			Doc: "Lifetime of a token for a hosted agent; the hosting runner fetches a new one before it ends."},
	}
}

// agentDefinitionConfigKeys returns the keys of the definitions of built-in agents from
// docs/spec/26-builtin-agents.md "Configuration values of this chapter" that the server reads so far
// (A1-02): the limits of agent_definition.v1 and the commands of a stdio MCP server. The maxima
// keep a definition far below the JSON body limit of a request; zero for a count allows none.
func agentDefinitionConfigKeys() []config.Key {
	return []config.Key{
		{Name: "agents.definition.instructions_max_bytes", Kind: config.KindInt, Default: "65536", Min: "1", Max: "1048576", Chapter: chapter26,
			Doc: "Longest instructions of a definition of a built-in agent in bytes."},
		{Name: "agents.definition.max_skills", Kind: config.KindInt, Default: "32", Min: "0", Max: "1024", Chapter: chapter26,
			Doc: "Most skills of a definition."},
		{Name: "agents.definition.skill_description_max_chars", Kind: config.KindInt, Default: "300", Min: "1", Max: "4096", Chapter: chapter26,
			Doc: "Longest skill description in characters; it is one line."},
		{Name: "agents.definition.skill_body_max_bytes", Kind: config.KindInt, Default: "65536", Min: "1", Max: "1048576", Chapter: chapter26,
			Doc: "Longest skill body and extra skill file in bytes."},
		{Name: "agents.definition.max_skill_files", Kind: config.KindInt, Default: "8", Min: "0", Max: "256", Chapter: chapter26,
			Doc: "Most extra files of one skill."},
		{Name: "agents.definition.max_mcp_servers", Kind: config.KindInt, Default: "16", Min: "0", Max: "256", Chapter: chapter26,
			Doc: "Most MCP servers of a definition."},
		{Name: "agents.definition.max_allowed_tools", Kind: config.KindInt, Default: "64", Min: "0", Max: "1024", Chapter: chapter26,
			Doc: "Most entries of settings.allowed_tools of a definition."},
		{Name: "agents.definition.max_cost_usd_per_run", Kind: config.KindFloat, Default: "50", Min: "0", Chapter: chapter26,
			Doc: "Highest cost cap per run in USD that a definition may set."},
		{Name: "agents.mcp_stdio_runners", Kind: config.KindStringList, Default: `["npx", "uvx"]`, Chapter: chapter26,
			Doc: "Commands a stdio MCP server of a definition may use."},
	}
}

// keyHostedTokenTTL is agents.hosted_token_ttl, which the cross-check
// hosted_token_ttl_within_api_token_max_ttl compares with auth.api_token_max_ttl.
const keyHostedTokenTTL = "agents.hosted_token_ttl"

// keyInterviewTimeout is agents.interview.timeout, which the deadline of interviewAgent adds to
// http.request_timeout (InterviewDeadline).
const keyInterviewTimeout = "agents.interview.timeout"

// agentInterviewConfigKeys returns the keys of the interview of "New agent"
// (docs/spec/26-builtin-agents.md 6.1 and 14, A1-07). agents.builder_agent_id is off by default:
// without it every interview falls back with builder_not_configured. The maximum of max_turns is
// the maxItems of AgentInterviewRequest.turns in docs/api/openapi.yaml, so every configured value
// can be sent. No cross-check against http.request_timeout: interviewAgent has its own deadline.
func agentInterviewConfigKeys() []config.Key {
	return []config.Key{
		{Name: "agents.builder_agent_id", Kind: config.KindString, Default: "", Chapter: chapter26,
			Doc: "Agent that answers interview requests (its id); empty means no interview, templates and the manual form only."},
		{Name: keyInterviewTimeout, Kind: config.KindDuration, Default: "90s", Min: minDeadline, Chapter: chapter26,
			Doc: "Longest wait of interviewAgent for the builder's answer; that request's deadline is this plus http.request_timeout."},
		{Name: "agents.interview.max_turns", Kind: config.KindInt, Default: "12", Min: "1", Max: "64", Chapter: chapter26,
			Doc: "Most turns of the interview history in one request."},
		{Name: "agents.interview.answer_max_chars", Kind: config.KindInt, Default: "4000", Min: "1", Max: "16000", Chapter: chapter26,
			Doc: "Longest turn of the interview in characters, and longest question of the builder."},
	}
}

// keyPresenceTimeout is agent.presence_timeout, which the cross-check
// presence_timeout_above_heartbeat_interval compares with sync.heartbeat_interval.
const keyPresenceTimeout = "agent.presence_timeout"
