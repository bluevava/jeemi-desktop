//go:build windows

package winauth

// These functions are shared with the separately linked service/installer.
// They are Go internals, never Wails bindings or parameters accepted from UI.
var (
	ServiceName           = serviceName
	ServiceDirectory      = serviceDirectory
	ServiceBinary         = serviceBinary
	ServiceCommand        = serviceCommand
	ServiceCommandMatches = serviceCommandMatches
	PipeName              = pipeName
	ValidSID              = validSID
	OwnerSID              = ownerSID
	EnsureStorage         = ensureStorage
	TrustedPath           = trustedPath
	FileDigest            = fileDigest
	ProcessHandleSID      = processHandleSID
	LockFile              = lockFile
	DecodeRequest         = decodeRequest
	Failure               = failure
)
