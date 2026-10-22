// SPDX-License-Identifier: Apache-2.0

/// The fake server of the client integration tests (brief M4-17; 15 20 row "Integration").
///
/// A separate library, never exported by `package:sync_engine/sync_engine.dart`, and imported
/// only from `test/` and `integration_test/` folders (tools/arch-check rule 10). The headless
/// tests of this package and the app tests from M4-18 on share it, so there is one fake that
/// cannot drift from the other (D-W-114).
///
/// [FakeServer.start] binds `127.0.0.1` with a free port, serves REST under `/api/v1/` and the
/// gateway at `/api/v1/gateway`, keeps a [FakeWorld] in memory ([WorldSeed.standard] fills it
/// with Alex, Sam and Robin), checks DPoP proofs ([DpopVerifier]), validates every request,
/// response and frame against `docs/api/` ([ContractSchemas]) and injects the faults of its
/// [FaultPlan]. [ScriptedAgent] plays the agent run of 04 16.5; `scenarios.dart` holds the
/// five flows of 04 16 as drivers over a [ScenarioClient].
library;

export 'src/testing/action_catalog.dart';
export 'src/testing/audit_log.dart';
export 'src/testing/dpop_verifier.dart';
export 'src/testing/fake_agent.dart';
export 'src/testing/fake_host.dart';
export 'src/testing/fake_server.dart';
export 'src/testing/faults.dart';
export 'src/testing/gateway_fake.dart';
export 'src/testing/rest/access_handlers.dart' show grantPreviewExample;
export 'src/testing/rest/agent_handlers.dart' show FakeI1Guard, i1Hint;
export 'src/testing/rest_fake.dart';
export 'src/testing/scenarios.dart';
export 'src/testing/schema_validator.dart';
export 'src/testing/scripted_agent.dart';
export 'src/testing/seed.dart';
export 'src/testing/world.dart';
