// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"maps"
	"os"
	"slices"

	"go.opentelemetry.io/otelc/tool/ex"
	"go.opentelemetry.io/otelc/tool/internal/configuration"
	"go.opentelemetry.io/otelc/tool/internal/rule"
)

const instrumentationMetadataFilename = "metadata.yaml"

// validateSelectedConfigurationMetadata reads metadata.yaml only for the
// instrumentation modules that provided rules matched in this build. Metadata
// provenance is attached when rule files are loaded, so this works for every
// rule kind without relying on a hook package's resolved directory.
func (sp *setupPhase) validateSelectedConfigurationMetadata(matched []*rule.InstRuleSet) error {
	metadataPathsSet := make(map[string]struct{})
	for _, ruleset := range matched {
		if ruleset == nil {
			continue
		}
		for _, metadataPath := range ruleset.ConfigurationMetadataPaths() {
			metadataPathsSet[metadataPath] = struct{}{}
		}
	}
	metadataPaths := slices.Sorted(maps.Keys(metadataPathsSet))
	var metadataFiles []configuration.Metadata
	for _, metadataPath := range metadataPaths {
		data, err := os.ReadFile(metadataPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return ex.Wrapf(err, "read selected instrumentation metadata %s", metadataPath)
		}
		metadata, err := configuration.ParseMetadata(data, metadataPath)
		if err != nil {
			return err
		}
		metadataFiles = append(metadataFiles, metadata)
	}
	if len(metadataFiles) == 0 {
		return nil
	}

	definitions, err := configuration.CatalogDefinitions()
	if err != nil {
		return ex.Wrapf(err, "load pinned configuration catalog")
	}
	resolved, err := configuration.ResolveMetadata(metadataFiles, definitions)
	if err != nil {
		return err
	}
	sp.Info(
		"Validated selected instrumentation configuration metadata",
		"modules",
		len(metadataFiles),
		"configurations",
		len(resolved),
	)
	return nil
}
