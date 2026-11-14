package auth

import "bytes"

const (
	// SystemUserName owns projects bootstrapped from a Function CR instead of
	// an interactive portal session. It is intentionally not a passkey user.
	SystemUserName = "litefunctions-system"
)

var SystemUserID = []byte(SystemUserName)

func IsSystemUser(name string) bool {
	return name == SystemUserName
}

func IsSystemUserID(id []byte) bool {
	return bytes.Equal(id, SystemUserID)
}
