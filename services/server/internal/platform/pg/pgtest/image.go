// SPDX-License-Identifier: Apache-2.0

package pgtest

// Image is the PostgreSQL 18 image of the test containers, pinned by digest like the database of
// the local SonarQube (tools/sonarqube/compose.yaml), so every test run uses the same server.
// Renovate updates the tag and the digest through the custom manager in renovate.json, which
// reads the comment on the next line.
//
// renovate: datasource=docker depName=postgres
const Image = "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336"
