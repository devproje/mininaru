package modules

import (
	"context"
	"os"
)

type Permission int

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Permission  Permission
	Execute     func(ctx context.Context, arguments string) (string, error)
}

const (
	PermissionSafe Permission = iota
	PermissionDangerous
)

var workingRoot string

func (p Permission) String() string {
	switch p {
	case PermissionDangerous:
		return "dangerous"
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
	var root string

	root = workingRoot
	if root == "" {
		root, _ = os.Getwd()
	}

	return []Def{
		CurrentTime(),
		FileRead(root),
		FileWrite(root),
		BashExec(root),
		WebSearch(),
	}
}

func SafeTools() []Def {
	var tools []Def
	var def Def

	for _, def = range DefaultTools() {
		if def.Permission != PermissionSafe {
			continue
		}

		tools = append(tools, def)
	}

	return tools
}
