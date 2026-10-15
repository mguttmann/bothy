// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// problemType is one entry of x-problem-codes of the schema Problem (06 section 5, R-API-23):
// the documentation getProblemType serves (M1-46).
type problemType struct {
	Code, Title, Description, ClientAction string
	Status                                 int
}

// semverPattern is SemVer 2.0.0 without a leading v (the info.version of the contract).
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// problemCodePattern is the form of a problem code (parameter CodePath).
var problemCodePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// readMeta reads info.version and the x-problem-codes of components.schemas.Problem from the
// spec tree and records every problem in p. A spec without x-problem-codes has no problem types
// (the fixture of a negative case); a present list must be well formed: a code of the pattern of
// CodePath, given once, a status from 400 to 599, a title and a description; client_action is
// optional. The result is sorted by code.
func readMeta(specPath string, doc *yaml.Node, p *problemsError) (string, []problemType) {
	root := doc.Content[0]
	version := scalar(value(value(root, "info"), "version"))
	if !semverPattern.MatchString(version) {
		p.add(specPath, "info.version %q is not a semantic version without a leading v", version)
	}
	list := value(value(value(value(root, "components"), "schemas"), "Problem"), "x-problem-codes")
	if list == nil {
		return version, nil
	}
	if list.Kind != yaml.SequenceNode {
		p.add(specPath, "components.schemas.Problem.x-problem-codes is not a list")
		return version, nil
	}
	var out []problemType
	seen := map[string]bool{}
	for i, n := range items(list) {
		at := fmt.Sprintf("x-problem-codes[%d]", i)
		pt := problemType{
			Code: scalar(value(n, "code")), Title: scalar(value(n, "title")),
			Description: scalar(value(n, "description")), ClientAction: scalar(value(n, "client_action")),
		}
		status, err := strconv.Atoi(scalar(value(n, "status")))
		switch {
		case !problemCodePattern.MatchString(pt.Code):
			p.add(specPath, "%s: code %q does not match %s", at, pt.Code, problemCodePattern)
			continue
		case seen[pt.Code]:
			p.add(specPath, "%s: code %s is listed twice", at, pt.Code)
			continue
		case err != nil || status < 400 || status > 599:
			p.add(specPath, "%s: code %s has no status from 400 to 599", at, pt.Code)
			continue
		case pt.Title == "" || pt.Description == "":
			p.add(specPath, "%s: code %s needs a title and a description", at, pt.Code)
			continue
		}
		seen[pt.Code] = true
		pt.Status = status
		out = append(out, pt)
	}
	slices.SortFunc(out, func(a, b problemType) int { return strings.Compare(a.Code, b.Code) })
	return version, out
}

// readGatewayVersions returns info.x-protocol-versions of the AsyncAPI document at path: the
// gateway protocol versions discovery reports (04 section 6.1). The list must be non-empty and
// hold distinct positive integers; it is returned sorted.
func readGatewayVersions(path string) ([]int, error) {
	doc, _, err := readYAML(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	list := value(value(doc.Content[0], "info"), "x-protocol-versions")
	nodes := items(list)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("%s: info.x-protocol-versions is missing or empty", path)
	}
	out := make([]int, 0, len(nodes))
	for _, n := range nodes {
		v, err := strconv.Atoi(scalar(n))
		if err != nil || v < 1 || slices.Contains(out, v) {
			return nil, fmt.Errorf("%s: info.x-protocol-versions must hold distinct positive integers", path)
		}
		out = append(out, v)
	}
	slices.Sort(out)
	return out, nil
}
