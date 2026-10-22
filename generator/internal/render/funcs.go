package render

import "html/template"

// withSharedFuncs adds the helpers that every template may use, so each
// renderer picks them up with a single call.
func withSharedFuncs(fm template.FuncMap) template.FuncMap {
	fm["isTestnet"] = IsTestnet
	fm["networkName"] = func() string { return network }

	// coreVersion extracts the core block version from nVersion.  Bitmark packs
	// extra data into the upper bits of nVersion — bit 8 marks an auxPoW block
	// and bits 9-11 carry the algorithm ID — so a raw nVersion reads as a large
	// meaningless number (5968644, 3588, 1028 …).  The low byte is the actual
	// version.
	fm["coreVersion"] = func(nVersion int64) int64 { return nVersion & 255 }

	return fm
}
