// Package resourceimport implements the bulk YAML manifest importer and exporter
// for monitored resources. The flow is: Parse (strict YAML) → Validate
// (pure, per-row) → Import (resolve refs, create via ResourceService, all-or-nothing)
// with a paired Export that round-trips the current resource set back to a manifest.
package resourceimport

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ManifestVersion is the only manifest schema version supported in Phase 1.
const ManifestVersion = 1

// MaxManifestResources caps a single manifest to keep the import request bounded
const MaxManifestResources = 500

// Manifest is the parsed representation of an import document.
type Manifest struct {
	Version   int            `yaml:"version"`
	Defaults  *Defaults      `yaml:"defaults"`
	Resources []ResourceDecl `yaml:"resources"`
}

// Defaults are applied to any resource declaration that omits the field.
type Defaults struct {
	Interval             *int `yaml:"interval"`
	Timeout              *int `yaml:"timeout"`
	ConfirmationChecks   *int `yaml:"confirmation_checks"`
	ConfirmationInterval *int `yaml:"confirmation_interval"`
}

// ResourceDecl is a single manifest row describing one resource to create.
type ResourceDecl struct {
	Name                 string   `yaml:"name"`
	Type                 string   `yaml:"type"`
	Target               string   `yaml:"target"`
	Interval             *int     `yaml:"interval"`
	Timeout              *int     `yaml:"timeout"`
	ConfirmationChecks   *int     `yaml:"confirmation_checks"`
	ConfirmationInterval *int     `yaml:"confirmation_interval"`
	Keyword              *string  `yaml:"keyword"`
	KeywordMode          *string  `yaml:"keyword_mode"`
	ProtocolType         *string  `yaml:"protocol_type"`
	ProtocolPort         *int     `yaml:"protocol_port"`
	HeartbeatInterval    *int     `yaml:"heartbeat_interval"`
	HeartbeatGrace       *int     `yaml:"heartbeat_grace"`
	Tags                 []string `yaml:"tags"`
	Component            string   `yaml:"component"`
	NotificationChannels []string `yaml:"notification_channels"`
}

// rowError carries the manifest row index alongside a parse/validation error so
// the caller can produce a row-addressable report.
type rowError struct {
	Index int
	Err   error
}

func (e *rowError) Error() string {
	return fmt.Sprintf("row %d: %v", e.Index, e.Err)
}

// topLevel mirrors Manifest but keeps resources as raw nodes so each row can be
// decoded strictly and independently (row-addressable unknown-field errors).
type topLevel struct {
	Version   int         `yaml:"version"`
	Defaults  *Defaults   `yaml:"defaults"`
	Resources []yaml.Node `yaml:"resources"`
}

// Parse strictly decodes a YAML manifest. Unknown fields are rejected: unknown
// top-level keys fail the whole document; an unknown per-row key fails only that
// row (returned as *rowError so the report stays row-addressable). Spec 078 FR-010a.
func Parse(data []byte) (*Manifest, error) {
	var top topLevel
	if err := strictUnmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if top.Version != ManifestVersion {
		return nil, fmt.Errorf("unsupported manifest version %d (expected %d)", top.Version, ManifestVersion)
	}

	m := &Manifest{Version: top.Version, Defaults: top.Defaults}
	m.Resources = make([]ResourceDecl, 0, len(top.Resources))
	for i := range top.Resources {
		// Re-marshal the row node and strict-decode it so unknown per-row keys
		// become row-addressable errors (yaml.Node itself has no strict mode).
		raw, err := yaml.Marshal(&top.Resources[i])
		if err != nil {
			return nil, &rowError{Index: i, Err: fmt.Errorf("invalid row: %w", err)}
		}
		var decl ResourceDecl
		if err := strictUnmarshal(raw, &decl); err != nil {
			return nil, &rowError{Index: i, Err: fmt.Errorf("invalid row: %w", err)}
		}
		m.Resources = append(m.Resources, decl)
	}
	return m, nil
}

// strictUnmarshal decodes with KnownFields(true) so unknown keys error.
func strictUnmarshal(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(out)
}
