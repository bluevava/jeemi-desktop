//go:build windows

package daemon

import "jeemi/internal/platform/winauth"

type Target = winauth.Target
type Request = winauth.Request
type Reply = winauth.Reply
type Network = winauth.Network
type NetworkFactory = winauth.NetworkFactory

const maxRequestBytes = 1 << 20
const HelperName = winauth.HelperName
const Protocol = winauth.Protocol

var (
	serviceName           = winauth.ServiceName
	serviceDirectory      = winauth.ServiceDirectory
	serviceBinary         = winauth.ServiceBinary
	serviceCommand        = winauth.ServiceCommand
	serviceCommandMatches = winauth.ServiceCommandMatches
	pipeName              = winauth.PipeName
	validSID              = winauth.ValidSID
	ownerSID              = winauth.OwnerSID
	ensureStorage         = winauth.EnsureStorage
	trustedPath           = winauth.TrustedPath
	fileDigest            = winauth.FileDigest
	processHandleSID      = winauth.ProcessHandleSID
	lockFile              = winauth.LockFile
	decodeRequest         = winauth.DecodeRequest
	failure               = winauth.Failure
)
