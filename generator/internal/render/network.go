package render

import "strings"

// network identifies the chain this generator instance renders.  It is set
// once at start-up from the NETWORK environment variable and read by every
// template through the isTestnet / networkName functions, so one template set
// serves both the mainnet explorer and the testnet explorer.
var network = "mainnet"

// SetNetwork configures the chain label used by all templates.  Any value
// other than "mainnet" (case-insensitive) is treated as a test network: pages
// gain a TESTNET badge and drop mainnet-only links.
func SetNetwork(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "mainnet"
	}
	network = name
}

// IsTestnet reports whether this instance renders a non-mainnet chain.
func IsTestnet() bool { return network != "mainnet" }
