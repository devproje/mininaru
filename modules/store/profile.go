// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-only

package store

import (
	"errors"
	"os"
	"strings"

	"github.com/devproje/mininaru/util"
	"github.com/pelletier/go-toml/v2"
)

type Profile struct {
	Name  *string `toml:"name"`
	Model *string `toml:"model"`

	Soul *string `toml:"soul"`
}

const profilePath = "profile.toml"

func UpsertProfile(profile *Profile) error {
	var path string
	var buf []byte
	var origin Profile
	var obj Profile
	var providerName string
	var modelName string
	var valid bool
	var exists bool

	var err error

	path = util.Path(profilePath)

	buf, err = os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	err = toml.Unmarshal(buf, &origin)
	if err != nil {
		return err
	}

	if profile.Name != nil {
		obj.Name = profile.Name
	} else {
		obj.Name = origin.Name
	}

	if profile.Model != nil {
		providerName, modelName, valid = strings.Cut(*profile.Model, ":")
		if !valid || providerName == "" || modelName == "" ||
			strings.TrimSpace(providerName) != providerName || strings.TrimSpace(modelName) != modelName {
			return ErrModelNotFound
		}

		exists, err = providerExists(providerName)
		if err != nil {
			return err
		}
		if !exists {
			return ErrModelNotFound
		}

		obj.Model = profile.Model
	} else {
		obj.Model = origin.Model
	}

	if profile.Soul != nil {
		obj.Soul = profile.Soul
	} else {
		obj.Soul = origin.Soul
	}

	buf, err = toml.Marshal(obj)
	if err != nil {
		return err
	}

	err = util.WriteFileAtomic(path, buf, 0600)
	if err != nil {
		return err
	}

	return nil
}

func GetProfile() (Profile, error) {
	var path string
	var buf []byte
	var obj Profile

	var err error

	path = util.Path(profilePath)

	buf, err = os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}

	err = toml.Unmarshal(buf, &obj)
	if err != nil {
		return Profile{}, err
	}

	return obj, err
}
