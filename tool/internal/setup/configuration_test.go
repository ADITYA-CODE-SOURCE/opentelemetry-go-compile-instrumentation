// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otelc/tool/internal/rule"
)

func TestValidateSelectedConfigurationMetadata(t *testing.T) {
	module := makeConfigurationModule(t, "example.com/instrumentation", `configurations:
  - ref: general.http.client.request_captured_headers
`)
	matched := matchedConfigurationRules(module)
	sp := newTestSetupPhase()

	err := sp.validateSelectedConfigurationMetadata(matched)
	require.NoError(t, err)
}

func TestConfigurationMetadataTracksEveryMatchedRuleType(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "metadata.yaml")
	set := rule.NewInstRuleSet("example.com/app")
	raw := &rule.InstRawRule{}
	function := &rule.InstFuncRule{}
	structure := &rule.InstStructRule{}
	call := &rule.InstCallRule{}
	literal := &rule.InstLitRule{}
	directive := &rule.InstDirectiveRule{}
	declaration := &rule.InstDeclRule{}
	file := &rule.InstFileRule{}
	rules := []rule.InstRule{
		raw, function, structure, call, literal, directive, declaration, file,
	}
	setRuleMetadataPath(rules, metadataPath)
	set.AddRawRule("/app.go", raw)
	set.AddFuncRule("/app.go", function)
	set.AddStructRule("/app.go", structure)
	set.AddCallRule("/app.go", call)
	set.AddLitRule("/app.go", literal)
	set.AddDirectiveRule("/app.go", directive)
	set.AddDeclRule("/app.go", declaration)
	set.AddFileRule(file)

	assert.Equal(t, []string{metadataPath}, set.ConfigurationMetadataPaths())
}

func TestValidateSelectedConfigurationMetadataForCallOnlyInstrumentation(t *testing.T) {
	module := makeConfigurationModule(t, "example.com/callonly", `configurations:
  - ref: missing.definition
`)
	set := rule.NewInstRuleSet("example.com/app")
	set.AddCallRule("/app.go", &rule.InstCallRule{InstBaseRule: metadataSource(
		filepath.Join(module.dir, instrumentationMetadataFilename),
	)})

	err := newTestSetupPhase().validateSelectedConfigurationMetadata([]*rule.InstRuleSet{set})

	require.ErrorContains(t, err, `unknown configuration ref "missing.definition"`)
	require.ErrorContains(t, err, filepath.Join(module.dir, instrumentationMetadataFilename))
}

func TestValidateSelectedConfigurationMetadataUnknownReference(t *testing.T) {
	module := makeConfigurationModule(t, "example.com/instrumentation", `configurations:
  - ref: missing.definition
`)
	matched := matchedConfigurationRules(module)

	err := newTestSetupPhase().validateSelectedConfigurationMetadata(matched)
	require.ErrorContains(t, err, `unknown configuration ref "missing.definition"`)
	require.ErrorContains(t, err, filepath.Join(module.dir, "metadata.yaml"))
}

func TestValidateSelectedConfigurationMetadataSkipsUnselectedModules(t *testing.T) {
	selected := makeConfigurationModule(t, "example.com/selected", "")
	unselected := makeConfigurationModule(t, "example.com/unselected", `configurations:
  - ref: missing.definition
`)
	matched := matchedConfigurationRules(selected)
	assert.NotContains(
		t,
		matched[0].ConfigurationMetadataPaths(),
		filepath.Join(unselected.dir, instrumentationMetadataFilename),
	)

	err := newTestSetupPhase().validateSelectedConfigurationMetadata(matched)
	require.NoError(t, err)
}

func TestValidateSelectedConfigurationMetadataReportsMetadataReadErrors(t *testing.T) {
	module := makeConfigurationModule(t, "example.com/instrumentation", "")
	require.NoError(t, os.Mkdir(filepath.Join(module.dir, instrumentationMetadataFilename), 0o755))
	matched := matchedConfigurationRules(module)

	err := newTestSetupPhase().validateSelectedConfigurationMetadata(matched)

	require.ErrorContains(t, err, "read selected instrumentation metadata")
}

func TestValidateSelectedConfigurationMetadataReportsParseErrors(t *testing.T) {
	module := makeConfigurationModule(t, "example.com/instrumentation", "configurations: [\n")
	matched := matchedConfigurationRules(module)

	err := newTestSetupPhase().validateSelectedConfigurationMetadata(matched)

	require.ErrorContains(t, err, filepath.Join(module.dir, instrumentationMetadataFilename))
	require.ErrorContains(t, err, "parse")
}

func TestValidateSelectedConfigurationMetadataWithNoMatchedHooks(t *testing.T) {
	sp := newTestSetupPhase()

	err := sp.validateSelectedConfigurationMetadata(nil)

	require.NoError(t, err)
}

func TestConfigurationMetadataPathsAreSetupOnly(t *testing.T) {
	set := rule.NewInstRuleSet("example.com/app")
	var base rule.InstBaseRule
	base.SetMetadataPath("/module/metadata.yaml")
	r := &rule.InstCallRule{InstBaseRule: base}
	set.AddCallRule("/app.go", r)

	encoded, err := json.Marshal(set)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "metadata.yaml")
}

func TestWarnIfNoInstrumentationMatched(t *testing.T) {
	var logs bytes.Buffer
	sp := &setupPhase{logger: slog.New(slog.NewTextHandler(&logs, nil))}

	sp.warnIfNoInstrumentationMatched(nil)

	assert.Contains(t, logs.String(), "no instrumentation rules matched any dependencies")
}

type configurationTestModule struct {
	dir      string
	hookPath string
}

func matchedConfigurationRules(module configurationTestModule) []*rule.InstRuleSet {
	set := rule.NewInstRuleSet("example.com/app")
	set.AddFuncRule(filepath.Join(module.dir, "http.go"), &rule.InstFuncRule{
		InstBaseRule: metadataSource(filepath.Join(module.dir, instrumentationMetadataFilename)),
		Path:         module.hookPath,
	})
	return []*rule.InstRuleSet{set}
}

func makeConfigurationModule(t *testing.T, modulePath, metadata string) configurationTestModule {
	t.Helper()
	moduleDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(moduleDir, "go.mod"),
		[]byte("module "+modulePath+"\n\ngo 1.26.0\n"),
		0o644,
	))
	hooksDir := filepath.Join(moduleDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "hooks.go"), []byte("package hooks\n"), 0o644))
	if metadata != "" {
		require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "metadata.yaml"), []byte(metadata), 0o644))
	}
	return configurationTestModule{dir: moduleDir, hookPath: modulePath + "/hooks"}
}

// vendoredHook is the application of makeVendoredHook.
type vendoredHook struct {
	appDir  string
	hookDir string
}

// makeVendoredHook creates an application (with a go.mod and a metadata.yaml)
// that vendors the module "example.com/hooks" and has the hook package
// "example.com/hooks/hooks". Like "go mod vendor", it does not create a go.mod
// in vendor/. It returns the application directory and the hook package
// directory.
func makeVendoredHook(t *testing.T, appMetadata, hookMetadata string) vendoredHook {
	t.Helper()
	appDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module example.com/app\n"), 0o644))
	if appMetadata != "" {
		require.NoError(t, os.WriteFile(
			filepath.Join(appDir, instrumentationMetadataFilename), []byte(appMetadata), 0o644,
		))
	}
	vendorDir := filepath.Join(appDir, "vendor")
	hookDir := filepath.Join(vendorDir, "example.com", "hooks", "hooks")
	require.NoError(t, os.MkdirAll(hookDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hookDir, "hooks.go"), []byte("package hooks\n"), 0o644))
	if hookMetadata != "" {
		require.NoError(t, os.WriteFile(
			filepath.Join(vendorDir, "example.com", "hooks", instrumentationMetadataFilename),
			[]byte(hookMetadata), 0o644,
		))
	}
	modulesTxt := "# example.com/hooks v1.0.0\n## explicit; go 1.26.0\nexample.com/hooks/hooks\n"
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "modules.txt"), []byte(modulesTxt), 0o644))
	return vendoredHook{appDir: appDir, hookDir: hookDir}
}

func TestValidateSelectedConfigurationMetadataVendoredHookIgnoresApplicationMetadata(t *testing.T) {
	// The metadata.yaml of the application is not an otelc metadata file, and
	// must not be read in place of the one of the hook module.
	hookDir := makeVendoredHook(t, "configurations: {}\n", "").hookDir
	set := rule.NewInstRuleSet("example.com/app")
	set.AddFuncRule(filepath.Join(hookDir, "hooks.go"), &rule.InstFuncRule{
		Path:         "example.com/hooks/hooks",
		InstBaseRule: metadataSource(filepath.Join(filepath.Dir(hookDir), instrumentationMetadataFilename)),
	})

	require.NoError(t, newTestSetupPhase().validateSelectedConfigurationMetadata([]*rule.InstRuleSet{set}))
}

func TestValidateSelectedConfigurationMetadataVendoredHookReadsModuleMetadata(t *testing.T) {
	hookDir := makeVendoredHook(t, "", "configurations:\n  - ref: missing.definition\n").hookDir
	set := rule.NewInstRuleSet("example.com/app")
	set.AddFuncRule(filepath.Join(hookDir, "hooks.go"), &rule.InstFuncRule{
		Path:         "example.com/hooks/hooks",
		InstBaseRule: metadataSource(filepath.Join(filepath.Dir(hookDir), instrumentationMetadataFilename)),
	})

	err := newTestSetupPhase().validateSelectedConfigurationMetadata([]*rule.InstRuleSet{set})

	require.ErrorContains(t, err, `unknown configuration ref "missing.definition"`)
}

func metadataSource(path string) rule.InstBaseRule {
	var base rule.InstBaseRule
	base.SetMetadataPath(path)
	return base
}
