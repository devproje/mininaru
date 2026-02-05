package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type VersionInfo struct {
	Version   string
	Branch    string
	GitHash   string
	BuildTime string
	GoVersion string
}

type ConfigData struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
	SSL  bool   `toml:"ssl"`

	Ver *VersionInfo `toml:"-"`
}

var (
	Get *ConfigData
)

func Load(ver *VersionInfo) error {
	var buf, err = os.ReadFile("config.toml")
	if err != nil {
		return err
	}

	err = toml.Unmarshal(buf, &Get)
	if err != nil {
		return err
	}

	Get.Ver = ver
	return nil
}
