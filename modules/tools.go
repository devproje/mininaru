// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package modules

import (
	"context"
)

type Permission int

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Permission  Permission
	Execute     func(ctx context.Context, arguments string) (string, error)
	daemon      bool
}

const (
	PermissionSafe Permission = iota
	PermissionDangerous
	PermissionPrivileged
)

var workingRoot string

func (p Permission) String() string {
	switch p {
	case PermissionDangerous:
		return "dangerous"
	case PermissionPrivileged:
		return "privileged"
	default:
		return "safe"
	}
}

func SetWorkingRoot(root string) error {
	var resolved string

	var err error

	resolved, err = toolRoot(root)
	if err != nil {
		return err
	}

	workingRoot = resolved
	return nil
}

func DefaultTools() []Def {
	var tools []Def

	tools = append(tools, builtinDefs()...)
	tools = append(tools, manager.defs()...)

	return tools
}

func DefaultToolsAt(root string) ([]Def, error) {
	var resolved string
	var rooted []Def
	var replacement map[string]Def
	var tools []Def
	var index int

	var err error

	resolved, err = toolRoot(root)
	if err != nil {
		return nil, err
	}
	rooted = []Def{FileRead(resolved), FileWrite(resolved), FileEdit(resolved), Glob(resolved), Grep(resolved), BashExec(resolved)}
	replacement = make(map[string]Def)
	for index = range rooted {
		replacement[rooted[index].Name] = rooted[index]
	}
	tools = DefaultTools()
	for index = range tools {
		if replacement[tools[index].Name].Name != "" {
			tools[index] = replacement[tools[index].Name]
		}
	}

	return tools, nil
}

func SafeTools() []Def {
	var def Def
	var tools []Def

	for _, def = range DefaultTools() {
		if def.Permission != PermissionSafe || !def.daemon {
			continue
		}

		tools = append(tools, def)
	}

	return tools
}
