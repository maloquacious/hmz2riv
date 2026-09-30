// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2riv

import (
	"github.com/maloquacious/semver"
)

var (
	version = semver.Version{
		Major: 0,
		Minor: 2,
		Patch: 0,
	}
)

func Version() semver.Version {
	return version
}
