// Package ruleset embeds the Netgen Betterleaks ruleset into the ngcheck binary.
package ruleset

import _ "embed"

// TOML is the content of betterleaks.toml.
//
//go:embed betterleaks.toml
var TOML []byte
