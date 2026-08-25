// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
)

type Permission int

const (
	PermissionSafe Permission = iota
	PermissionDangerous
)

func (p Permission) String() string {
	switch p {
	case PermissionDangerous:
		return "dangerous"
	default:
		return "safe"
	}
}

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Permission  Permission
	Execute     func(ctx context.Context, arguments string) (string, error)
}
