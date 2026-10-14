# SPDX-License-Identifier: Apache-2.0
#
# Gate G4 of the Dart API client (docs/spec/06-api.md 15; root target `api-clients`). Skeleton of
# M4-01: no client is generated yet, and M4-02 replaces this target with the generation from
# docs/api/openapi.yaml, the compile and the comparison (docs/DECISIONS.md D-M4-01-11). Until
# then the target fails when generated code is tracked here, because nothing would keep it equal
# to the contract. git runs without the variables of a hook, so it finds this repository from
# the directory, as the root recipe does.

SHELL := bash

.PHONY: generate-check

generate-check: ## Skeleton: no generated code may be tracked before the generator (M4-02)
	@files=$$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_COMMON_DIR -u GIT_PREFIX \
	  git ls-files -- lib/src/generated '*.g.dart') || exit 1; \
	if [ -n "$$files" ]; then \
	  echo "api_client: generated code is tracked, but its generator comes with M4-02:" >&2; \
	  printf '%s\n' "$$files" >&2; exit 1; fi; \
	echo "  api_client: skeleton (M4-01), nothing generated yet; M4-02 adds the generation"
