// +build !linux

package main

import "time"

func maybeSetLinkMtime(destPath string, t time.Time) error {
	return nil
}
