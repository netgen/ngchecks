// Package ruleset embeds the built-in Betterleaks ruleset into the ngchecks binary.
package ruleset

import _ "embed"

// TOML is the content of betterleaks.toml.
//
//go:embed betterleaks.toml
var TOML []byte
