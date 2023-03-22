package util

import "fmt"

// IPAddressClaimName returns a name given a VsphereVM name, deviceIndex, and
// poolIndex.
func IPAddressClaimName(vmName string, deviceIndex, poolIndex int) string {
	return fmt.Sprintf("%s-%d-%d", vmName, deviceIndex, poolIndex)
}
