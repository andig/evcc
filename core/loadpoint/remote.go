package loadpoint

import "strings"

// RemoteDemand defines external status demand
type RemoteDemand string

// remote status demand definition
const (
	RemoteEnable      RemoteDemand = ""
	RemoteHardDisable RemoteDemand = "hard"
	RemoteSoftDisable RemoteDemand = "soft"
)

// RemoteDemandString converts string to RemoteDemand
func RemoteDemandString(demand string) (RemoteDemand, error) {
	switch strings.ToLower(demand) {
	case string(RemoteHardDisable):
		return RemoteHardDisable, nil
	case string(RemoteSoftDisable):
		return RemoteSoftDisable, nil
	default:
		return RemoteEnable, nil
	}
}
