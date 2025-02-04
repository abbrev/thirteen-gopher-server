//go:build unix
// Copyright 2025 Christopher Williams
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"os/user"
	"strconv"
	"syscall"
)

const (
	safePath        = "/usr/bin:/bin"
	defaultSiteRoot = "/srv/gopher"
)

func changeUser(username string) error {
	if username != "" {
		u, err := user.Lookup(username)
		if err != nil {
			return err
		}
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return err
		}
		err = syscall.Setuid(uid)
		if err != nil {
			return err
		}
	}
	return nil
}
