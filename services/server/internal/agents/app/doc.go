// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the agents module (M1-51): one per operation of the group
// Agents in docs/spec/06-api.md section 16. Each loads its resource, calls authorize with the
// action of docs/spec/05-actions.md section 4.10, runs its writes in one unit of work of
// kernel/txn with the allow audit entry and agent.updated, and declares the rights change where
// the column "Rechte" says ja, so the rights hook of authz raises permission_version and checks
// I1 before the commit. The outgoing ports are Store (the table agents), Cursors and the ports of
// identity (PrincipalWriter, Directory, the token revocation).
//
// Example:
//
//	svc, err := app.New(app.Config{PageSizeDefault: 50, PageSizeMax: 200}, app.Deps{...})
//	agent, err := svc.SetBillingMode(ctx, caller, agentID, agents.BillingModeCmd{
//		BillingMode: agents.BillingOrganization, Reason: "Switch to the company key",
//		IfMatch: agents.Precondition{Present: true, Valid: true, Version: 3},
//	})
package app
