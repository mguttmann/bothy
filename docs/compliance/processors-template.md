# Template: processors and recipients

Status: template M0. For instance operators. Not legal advice.

Purpose: a list of all services to which an instance passes personal data, with what the product
sends there. Basis for contracts under Art. 28 GDPR, for the record
([ropa-template.md](ropa-template.md), field "Recipients") and for transfers to third countries.

How to use the template:

1. Copy the file into your own documentation.
2. Fill in one row in section 2 per service that is actually used; delete rows that are not used.
   The column "What the product sends" describes the product and applies to every instance.
3. The instance operator assesses the role (processor, independent controller, not a recipient).
4. Update the list for every new agent, every new app and every new provider.

## 1. What the product passes to services

| Service type | What the product sends | What it never sends | Source |
|---|---|---|---|
| Push (APNs, FCM, WNS) | push token, app identifier, IDs of thread and message, neutral placeholder text, time | message contents, names, files | ADR 0031, [../spec/13-push-notifications.md](../spec/13-push-notifications.md) |
| Identity provider (OIDC) | login request with `state`, `nonce`, PKCE; the provider returns subject, name, optionally email and groups | contents | [../spec/08-identity.md](../spec/08-identity.md) |
| LLM provider (through the agent runtime, not through the server) | thread contents, earlier messages up to `history_backfill_limit`, files, authors as handles, channel prompt | credentials of other people; email addresses | [../spec/20-privacy.md](../spec/20-privacy.md), section 6 |
| Telemetry backend | metrics, traces, logs without contents, pseudonymous IDs | contents, IP addresses, names | [../spec/20-privacy.md](../spec/20-privacy.md), section 7 |
| Crash reports of the apps | sanitized stack traces, app version, platform | contents, tokens, names | [../spec/19-security.md](../spec/19-security.md), SEC-LOG-7 |
| Object storage in the cloud | files, thumbnails, export packages, deletion journal | | [../spec/12-files.md](../spec/12-files.md) |
| Hosting (cloud VM, managed database) | everything the instance stores | | [../spec/16-operations.md](../spec/16-operations.md) |
| Transcription service | audio of voice messages, language | names, other contents | [../spec/12-files.md](../spec/12-files.md) |
| Apps and integrations | events of their subscriptions within scopes and grants | whatever their grants do not allow | [../spec/07-permissions.md](../spec/07-permissions.md), section 2.10 |
| Backup target | encrypted images | keys of the backup | [../spec/19-security.md](../spec/19-security.md), SEC-BAK-* |

Not a recipient in the sense of this list: the virus scanner, as long as it runs locally (it only
downloads signatures); the maker of the product.

## 2. List

| Service | Provider | Role | Purpose | Data | Data subjects | Place of processing | Third-country safeguard | Contract (DPA) | Subprocessors | Deletion period at the provider | Checked on |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Push iOS, macOS | `<Apple>` | `<...>` | notification | see 1 | members with Apple devices | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Push Android | `<Google Firebase>` | `<...>` | notification | see 1 | members with Android devices | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Push Windows (if used) | `<Microsoft>` | `<...>` | notification | see 1 | members with Windows | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Identity provider | `<for example Microsoft Entra ID>` | `<...>` | login | see 1 | all members | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| LLM provider per agent | `<for example Anthropic>` | `<...>` | agent processing | see 1 | see record V2 | `<region>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Agent runtime, if not self-operated | `<...>` | `<...>` | execution of the agents | see 1 | see V2 | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Telemetry | `<for example self-operated Grafana stack, New Relic, Datadog, Azure Monitor>` | `<...>` | operations | see 1 | members (pseudonymous) | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Crash reports | `<for example Sentry or "off">` | `<...>` | debugging of the apps | see 1 | members | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Hosting | `<provider or own operation>` | `<...>` | operation of the instance | all | all | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Object storage | `<provider or local>` | `<...>` | files | contents | all | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Transcription | `<provider or local>` | `<...>` | voice messages | audio | speakers | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| Backup target | `<provider or location>` | `<...>` | backup | all, encrypted | all | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |
| App `<name>` | `<provider of the app>` | `<...>` | `<...>` | subscriptions: `<event types>`, scopes: `<...>` | `<...>` | `<...>` | `<...>` | `<link>` | `<link>` | `<...>` | `<date>` |

## 3. Example with public information: Anthropic API

For orientation only; check again before use. All information retrieved 2026-10-10.

| Field | Information | Source |
|---|---|---|
| Contract | The Data Processing Addendum is part of the Commercial Terms by reference; the customer is the controller, Anthropic the processor | https://www.anthropic.com/legal/data-processing-addendum [verified] |
| Subprocessors | List with an objection period according to the DPA | https://www.anthropic.com/subprocessors [verified] |
| Training | "Anthropic may not train models on Customer Content from Services." | https://www.anthropic.com/legal/commercial-terms [verified] |
| Deletion period | API inputs and outputs "within 30 days"; in case of a policy violation up to 2 years | https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data [verified] |
| Region | `inference_geo` with the values `global` and `us`; no EU option named | https://platform.claude.com/docs/en/manage-claude/usage-cost-api [verified] |
| Notice to users | The customer informs users that outputs are not reliable without review | Commercial Terms, D.3 [verified] |
| Login | Personal subscriptions (OAuth) only for one's own, ordinary use; third-party products must not collect, store or relay Claude.ai credentials | https://code.claude.com/docs/en/legal-and-compliance [verified] |
