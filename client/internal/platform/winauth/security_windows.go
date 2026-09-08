//go:build windows

package winauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func ownerSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}
func validSID(sid string) bool {
	value, err := windows.StringToSid(sid)
	return err == nil && value.String() == sid && strings.HasPrefix(sid, "S-1-5-21-")
}
func serviceName(sid string) string {
	sum := sha256.Sum256([]byte(sid))
	return "JeemiAuthorization-" + hex.EncodeToString(sum[:8])
}
func pipeName(sid string) string { return `\\.\pipe\` + serviceName(sid) }
func serviceDirectory(sid string) (string, error) {
	if !validSID(sid) {
		return "", failure("authorization_identity_failed")
	}
	base, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "JeemiAuthorization", sid), nil
}
func serviceBinary(sid string) (string, error) {
	directory, err := serviceDirectory(sid)
	return filepath.Join(directory, HelperName), err
}

func secureDirectory(path, sddl string, create bool) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	if create {
		attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
		if err = windows.CreateDirectory(p, &attributes); err != nil && err != windows.ERROR_ALREADY_EXISTS {
			return err
		}
	}
	// Never repair ACLs on an attacker-created/reparse directory. Existing
	// storage has to be protected already before we place privileged files there.
	return trustedPath(path, true)
}

func trustedPath(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.IsDir() != directory || info.Mode()&os.ModeSymlink != 0 || (!directory && !info.Mode().IsRegular()) {
		return failure("authorization_identity_failed")
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := windows.GetFileAttributes(p)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return failure("authorization_identity_failed")
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	ownerText := owner.String()
	if ownerText != "S-1-5-18" && ownerText != "S-1-5-32-544" {
		return failure("authorization_identity_failed")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return failure("authorization_identity_failed")
	}
	// Reject write/delete/ACL ownership rights for anyone except SYSTEM/admins.
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err = windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if sid == "S-1-5-18" || sid == "S-1-5-32-544" {
			continue
		}
		const writes = windows.GENERIC_ALL | windows.GENERIC_WRITE | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES | 0x40 // FILE_DELETE_CHILD
		if ace.Mask&writes != 0 {
			return failure("authorization_identity_failed")
		}
	}
	return nil
}

func ensureStorage(sid string) (string, error) {
	root, err := serviceDirectory(sid)
	if err != nil {
		return "", err
	}
	if err = secureDirectory(filepath.Dir(root), "O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;BU)", true); err != nil {
		return "", err
	}
	if err = secureDirectory(root, "O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;"+sid+")", true); err != nil {
		return "", err
	}
	return root, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 64 || info.Size() > 512<<20 {
		return "", failure("authorization_identity_failed")
	}
	hash := sha256.New()
	if _, err = io.CopyN(hash, file, info.Size()); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func processHandleSID(process windows.Handle) (string, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}
func lockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func redacted(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("authorization_service_failed")
}
