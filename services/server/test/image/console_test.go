// SPDX-License-Identifier: Apache-2.0

//go:build imagetest

package image_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// consoleDir is where the console stage copies `flutter build web` and what the image's
// BOTHY_CONSOLE__ASSETS_DIR names (brief M6-02 Design note 4).
const consoleDir = "usr/share/bothy/console"

// keepContent are the files whose content exportFS keeps besides the binary.
var keepContent = map[string]bool{
	consoleDir + "/index.html":   true,
	consoleDir + "/version.json": true,
}

// TestConsoleInImage: the image carries the console build at the path its default
// configuration names in console.assets_dir, so the api process serves index.html under
// /console/ (the serving itself is TestConsoleServing of internal/platform/httpserver); its
// version.json names the API version the server reports, and the build is owned by root and not
// writable for the running user.
func TestConsoleInImage(t *testing.T) {
	image := env(t, "BOTHY_TEST_IMAGE")
	if want := "BOTHY_CONSOLE__ASSETS_DIR=/" + consoleDir; !slices.Contains(inspect(t, image).Config.Env, want) {
		t.Errorf("image environment lacks %s", want)
	}
	fs := exportFS(t, image)
	index, ok := fs[consoleDir+"/index.html"]
	if !ok {
		t.Fatalf("%s/index.html is missing", consoleDir)
	}
	if !strings.Contains(string(index.data), `<base href="/console/">`) {
		t.Errorf("index.html has no base href /console/")
	}
	for _, name := range []string{"main.dart.js", "flutter_bootstrap.js", "canvaskit/canvaskit.wasm"} {
		if _, ok := fs[consoleDir+"/"+name]; !ok {
			t.Errorf("%s/%s is missing", consoleDir, name)
		}
	}
	var v struct {
		SpecVersion string `json:"spec_version"`
	}
	if err := json.Unmarshal(fs[consoleDir+"/version.json"].data, &v); err != nil || v.SpecVersion != routes.SpecVersion {
		t.Errorf("version.json spec_version = %q (%v), want %q", v.SpecVersion, err, routes.SpecVersion)
	}
	for name, e := range fs {
		if !strings.HasPrefix(name, consoleDir) {
			continue
		}
		if h := e.header; h.Uid != 0 || h.Gid != 0 || h.Mode&0o022 != 0 {
			t.Errorf("%s: owner %d:%d mode %o, want root and not writable for others", name, h.Uid, h.Gid, h.Mode&0o7777)
		}
	}
}
