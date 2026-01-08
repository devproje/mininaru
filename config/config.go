package config

type VersionInfo struct {
	Version   string
	Branch    string
	GitHash   string
	BuildTime string
	GoVersion string
}

type ConfigData struct {
	Ver *VersionInfo
}

var (
	Get *ConfigData
)

func Load(ver *VersionInfo) {
	Get = &ConfigData{
		Ver: ver,
	}
}
