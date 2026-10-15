# SPDX-License-Identifier: Apache-2.0
#
# Generation of the Dart REST client from docs/api/openapi.yaml and gate G4 for Dart
# (docs/spec/06-api.md 14 and 15; the root target `api-clients` calls `generate-check`).
# The recipe lives in tool/generate.sh; this file pins the generator (docs/DECISIONS.md
# D-M4-02-1): openapi-generator 7.26.0 by the digest of its multi-architecture index, run
# without network.

SHELL := bash

GENERATOR_IMAGE := openapitools/openapi-generator-cli:v7.26.0@sha256:a304ddf1e2e5f24f68fa3153568d6174cea4959d09aa8e3db6d526fd0782326d
# No network for the generator: it reads the spec from the read-only mount and nothing else.
GENERATOR_DOCKER_FLAGS := --rm --network none
export GENERATOR_IMAGE GENERATOR_DOCKER_FLAGS

.PHONY: generate generate-check

generate: ## Regenerate lib/src/generated/ from docs/api/openapi.yaml (needs Docker and Flutter)
	@tool/generate.sh generate

generate-check: ## G4: a fresh generation in a temporary copy equals lib/src/generated/ and analyses
	@tool/generate.sh check
