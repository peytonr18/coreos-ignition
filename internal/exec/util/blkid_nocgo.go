//go:build linux && !cgo

package util

import "errors"

// GetBlockDevices returns an error when built without cgo.
func GetBlockDevices(fstype string) ([]string, error) {
	return nil, errors.New("blkid support requires cgo")
}

