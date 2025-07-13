package packager

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/richardwilkes/toolbox/v2/xfilepath"
)

// FileData holds information about a file type for macOS.
type FileData struct {
	Name       string   `yaml:"name"`
	Icon       string   `yaml:"icon"`
	IconName   string   `yaml:"-"`
	Role       string   `yaml:"role"`
	Rank       string   `yaml:"rank"`
	UTI        string   `yaml:"uti"`
	ConformsTo []string `yaml:"conforms_to"`
	Extensions []string `yaml:"extensions"`
	MimeTypes  []string `yaml:"mime_types"`
}

// MacCodeSigning holds information about code signing for macOS.
type MacCodeSigning struct {
	Identity    string   `yaml:"identity"`
	Credentials string   `yaml:"credentials"`
	Options     []string `yaml:"options"`
}

// MacOnlyOpts holds options for macOS.
type MacOnlyOpts struct {
	FinderAppName             string         `yaml:"finder_app_name"`
	AppID                     string         `yaml:"app_id"`
	MinimumSystemVersionAMD64 string         `yaml:"minimum_system_version_amd64"`
	MinimumSystemVersionARM64 string         `yaml:"minimum_system_version_arm64"`
	CategoryUTI               string         `yaml:"category_uti"`
	CodeSigning               MacCodeSigning `yaml:"code_signing"`
}

// Config holds the configuration for the packager.
type Config struct {
	version         string
	FullName        string      `yaml:"full_name"`
	ExecutableName  string      `yaml:"executable_name"`
	AppIcon         string      `yaml:"app_icon"`
	Description     string      `yaml:"description"`
	CopyrightHolder string      `yaml:"copyright_holder"`
	CopyrightYears  string      `yaml:"copyright_years"`
	Trademarks      string      `yaml:"trademarks"`
	FileInfo        []*FileData `yaml:"file_info"`
	Mac             MacOnlyOpts `yaml:"mac"`
}

func (c *Config) prepare(version string) {
	c.version = version
	c.ExecutableName = xfilepath.BaseName(c.ExecutableName)
}

func (c *Config) finderAppName() string { //nolint:unused // This is used only on some platforms
	if c.Mac.FinderAppName != "" {
		return c.Mac.FinderAppName
	}
	return c.ExecutableName
}

func (c *Config) shortAppVersion() string { //nolint:unused // This is used only on some platforms
	shortVersion := strings.TrimSuffix(c.version, ".0")
	if strings.IndexByte(shortVersion, '.') == -1 {
		return c.version
	}
	return shortVersion
}

func (c *Config) macMinSysVersion() string { //nolint:unused // This is used only on some platforms
	if runtime.GOARCH == "arm64" {
		return c.Mac.MinimumSystemVersionARM64
	}
	return c.Mac.MinimumSystemVersionAMD64
}

func (c *Config) copyright() string { //nolint:unused // This is used only on some platforms
	return fmt.Sprintf("©%s by %s", c.CopyrightYears, c.CopyrightHolder)
}
