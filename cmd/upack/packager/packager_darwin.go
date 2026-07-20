// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xfilepath"
	"github.com/richardwilkes/toolbox/v2/ximage/icns"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
)

func prepareBinary(cfg *Config) error {
	return createAppPackage(cfg)
}

func createAppPackage(cfg *Config) error {
	appName := cfg.finderAppName() + ".app"
	if err := os.RemoveAll(appName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errs.Wrap(err)
	}
	contentsDir := filepath.Join(appName, "Contents")
	exeDir := filepath.Join(contentsDir, "MacOS")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		return errs.Wrap(err)
	}
	resDir := filepath.Join(contentsDir, "Resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return errs.Wrap(err)
	}
	if err := createICNS(cfg.AppIcon, resDir); err != nil {
		return err
	}
	for _, f := range cfg.FileInfo {
		// The plist references the file type's .icns (CFBundleTypeIconFile / UTTypeIconFile) for entries whose Rank
		// is Owner, so that is the predicate for generating them, too.
		if f.Rank == rankOwner {
			if err := createICNS(f.Icon, resDir); err != nil {
				return err
			}
		}
	}
	if err := copyFile(cfg.ExecutableName, filepath.Join(exeDir, cfg.ExecutableName), 0o755); err != nil {
		return err
	}
	return writePlist(cfg, filepath.Join(contentsDir, "Info.plist"))
}

func createICNS(srcIconPath, dstDirPath string) (err error) {
	var img image.Image
	if img, err = loadPNG(srcIconPath); err != nil {
		return err
	}
	var f *os.File
	f, err = os.Create(filepath.Join(dstDirPath, xfilepath.BaseName(srcIconPath)) + ".icns")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = errs.Wrap(cerr)
		}
	}()
	err = errs.Wrap(icns.Encode(f, img))
	return err
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer xio.CloseIgnoringErrors(f)
	return png.Decode(f)
}

func writePlist(cfg *Config, targetPath string) error {
	tmpl, err := template.New("plist").Parse(plistTmpl)
	if err != nil {
		return errs.Wrap(err)
	}
	for _, f := range cfg.FileInfo {
		f.IconName = xfilepath.BaseName(f.Icon) + ".icns"
	}
	exportCount := 0
	importCount := 0
	for _, one := range cfg.FileInfo {
		if one.Rank == rankOwner {
			exportCount++
		} else {
			importCount++
		}
	}
	type tmplData struct {
		FinderAppName        string
		AppCmdName           string
		SpokenName           string
		AppID                string
		AppIcon              string
		AppVersion           string
		ShortVersion         string
		MinimumSystemVersion string
		Copyright            string
		CategoryUTI          string
		FileInfo             []*FileData
		ExportCount          int
		ImportCount          int
	}
	var w bytes.Buffer
	if err = tmpl.Execute(&w, &tmplData{
		FinderAppName:        cfg.finderAppName(),
		AppCmdName:           cfg.ExecutableName,
		SpokenName:           cfg.FullName,
		AppID:                cfg.Mac.AppID,
		AppIcon:              xfilepath.BaseName(cfg.AppIcon) + ".icns",
		AppVersion:           cfg.version,
		ShortVersion:         cfg.shortAppVersion(),
		MinimumSystemVersion: cfg.macMinSysVersion(),
		Copyright:            cfg.copyright(),
		CategoryUTI:          cfg.Mac.CategoryUTI,
		FileInfo:             cfg.FileInfo,
		ExportCount:          exportCount,
		ImportCount:          importCount,
	}); err != nil {
		return errs.Wrap(err)
	}
	if err = os.WriteFile(targetPath, w.Bytes(), 0o644); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func generateDistribution(cfg *Config) error {
	dstPath := cfg.ExecutableName + "-" + cfg.version + "-macos-" + runtime.GOARCH + ".dmg"
	if xos.FileExists(dstPath) {
		if err := os.Remove(dstPath); err != nil {
			return errs.Wrap(err)
		}
	}
	if err := signApp(cfg); err != nil {
		return err
	}
	if err := createDiskImage(cfg, dstPath); err != nil {
		return err
	}
	return notarize(cfg, dstPath)
}

// notarize submits the distribution to Apple's notary service and staples the resulting ticket onto it. Credentials
// are passed directly to notarytool and nothing is persisted on the machine: an App Store Connect API key is used when
// supplied (in the configuration or via the NOTARY_API_KEY, NOTARY_API_KEY_ID, and NOTARY_API_KEY_ISSUER environment
// variables), otherwise a pre-existing keychain profile named by Credentials is used.
func notarize(cfg *Config, dstPath string) error {
	signing := &cfg.Mac.CodeSigning
	args := []string{"notarytool", "submit", dstPath, "--wait"}
	key := valueOrEnv(signing.APIKey, "NOTARY_API_KEY")
	keyID := valueOrEnv(signing.APIKeyID, "NOTARY_API_KEY_ID")
	issuer := valueOrEnv(signing.APIKeyIssuer, "NOTARY_API_KEY_ISSUER")
	switch {
	case key != "" && keyID != "" && issuer != "":
		args = append(args, "--key", key, "--key-id", keyID, "--issuer", issuer)
	case key != "" || keyID != "" || issuer != "":
		return errs.New("incomplete App Store Connect API key: the key file, key ID, and issuer ID must all be set")
	case signing.Credentials != "":
		args = append(args, "--keychain-profile", signing.Credentials)
	default:
		return errs.New("notarization requires either an App Store Connect API key or a credentials (keychain profile) name")
	}
	if err := run(exec.Command("xcrun", args...)); err != nil {
		return err
	}
	return run(exec.Command("xcrun", "stapler", "staple", dstPath))
}

func valueOrEnv(value, envVar string) string {
	if value != "" {
		return value
	}
	return os.Getenv(envVar)
}

func signApp(cfg *Config) error {
	var opts []string
	opts = append(opts,
		"-s", cfg.Mac.CodeSigning.Identity,
		"-f",
		"-v",
		"--timestamp",
	)
	if len(cfg.Mac.CodeSigning.Options) > 0 {
		opts = append(opts, "--options", strings.Join(cfg.Mac.CodeSigning.Options, ","))
	}
	opts = append(opts, cfg.finderAppName()+".app")
	return run(exec.Command("codesign", opts...))
}

func run(cmd *exec.Cmd) error {
	var wg sync.WaitGroup
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errs.Wrap(err)
	}
	wg.Go(func() { copyPipe(stdout, os.Stdout) })
	var stderr io.ReadCloser
	if stderr, err = cmd.StderrPipe(); err != nil {
		return errs.Wrap(err)
	}
	wg.Go(func() { copyPipe(stderr, os.Stderr) })
	if err = cmd.Start(); err != nil {
		return errs.Wrap(err)
	}
	wg.Wait()
	if err = cmd.Wait(); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func copyPipe(r io.Reader, w io.Writer) {
	if _, err := io.Copy(w, r); err != nil && !errors.Is(err, io.EOF) {
		slog.Warn("unable to copy pipe", "error", err)
	}
}

func createDiskImage(cfg *Config, dstPath string) error {
	var opts []string
	finderName := cfg.finderAppName()
	finderApp := finderName + ".app"
	opts = append(opts,
		"--volname", finderName+" v"+cfg.version,
		"--icon-size", "128",
		"--window-size", "448", "280",
		"--add-file", finderApp, finderApp, "64", "64",
		"--app-drop-link", "256", "64",
		"--codesign", cfg.Mac.CodeSigning.Identity,
		"--hdiutil-quiet",
		"--no-internet-enable",
		dstPath,
	)
	tmpDir, err := os.MkdirTemp(".", "tmp")
	if err != nil {
		return err
	}
	defer func() {
		if rerr := os.Remove(tmpDir); rerr != nil {
			slog.Warn("unable to remove temp dir", "error", rerr)
		}
	}()
	opts = append(opts, tmpDir)
	return run(exec.Command("create-dmg", opts...))
}

const plistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleName</key>
	<string>{{.FinderAppName | html}}</string>
	<key>CFBundleDisplayName</key>
	<string>{{.FinderAppName | html}}</string>
	<key>CFBundleIdentifier</key>
	<string>{{.AppID | html}}</string>
	<key>CFBundleVersion</key>
	<string>{{.AppVersion | html}}</string>
	<key>CFBundleShortVersionString</key>
	<string>{{.ShortVersion | html}}</string>
	<key>LSMinimumSystemVersion</key>
	<string>{{.MinimumSystemVersion | html}}</string>
	<key>CFBundleExecutable</key>
	<string>{{.AppCmdName | html}}</string>
	<key>NSHumanReadableCopyright</key>
	<string>{{.Copyright | html}}</string>
	<key>CFBundleDevelopmentRegion</key>
	<string>en-US</string>
	<key>CFBundleIconFile</key>
	<string>{{.AppIcon | html}}</string>
	<key>CFBundleSpokenName</key>
	<string>{{.SpokenName | html}}</string>
	<key>LSApplicationCategoryType</key>
	<string>{{.CategoryUTI | html}}</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
{{- if .FileInfo}}
	<key>CFBundleDocumentTypes</key>
	<array>
{{- range .FileInfo}}
		<dict>
			<key>CFBundleTypeName</key>
			<string>{{.Name | html}}</string>
{{- if eq .Rank "Owner"}}
			<key>CFBundleTypeIconFile</key>
			<string>{{.IconName | html}}</string>
{{- end}}
			<key>CFBundleTypeRole</key>
			<string>{{.Role | html}}</string>
			<key>LSHandlerRank</key>
			<string>{{.Rank | html}}</string>
			<key>LSItemContentTypes</key>
			<array>
				<string>{{.UTI | html}}</string>
			</array>
		</dict>
{{- end}}
	</array>
{{- if .ExportCount}}
	<key>UTExportedTypeDeclarations</key>
	<array>
{{- range .FileInfo}}
{{- if eq .Rank "Owner"}}
		<dict>
			<key>UTTypeIdentifier</key>
			<string>{{.UTI | html}}</string>
			<key>UTTypeDescription</key>
			<string>{{.Name | html}}</string>
			<key>UTTypeIconFile</key>
			<string>{{.IconName | html}}</string>
			<key>UTTypeConformsTo</key>
			<array>
{{- range .ConformsTo}}
				<string>{{. | html}}</string>
{{- end}}
			</array>
			<key>UTTypeTagSpecification</key>
			<dict>
				<key>public.filename-extension</key>
				<array>
{{- range .Extensions}}
					<string>{{. | html}}</string>
{{- end}}
				</array>
				<key>public.mime-type</key>
				<array>
{{- range .MimeTypes}}
					<string>{{. | html}}</string>
{{- end}}
				</array>
			</dict>
		</dict>
{{- end}}
{{- end}}
	</array>
{{- end}}
{{- if .ImportCount}}
	<key>UTImportedTypeDeclarations</key>
	<array>
{{- range .FileInfo}}
{{- if ne .Rank "Owner"}}
		<dict>
			<key>UTTypeIdentifier</key>
			<string>{{.UTI | html}}</string>
			<key>UTTypeDescription</key>
			<string>{{.Name | html}}</string>
			<key>UTTypeConformsTo</key>
			<array>
{{- range .ConformsTo}}
				<string>{{. | html}}</string>
{{- end}}
			</array>
			<key>UTTypeTagSpecification</key>
			<dict>
				<key>public.filename-extension</key>
				<array>
{{- range .Extensions}}
					<string>{{. | html}}</string>
{{- end}}
				</array>
				<key>public.mime-type</key>
				<array>
{{- range .MimeTypes}}
					<string>{{. | html}}</string>
{{- end}}
				</array>
			</dict>
		</dict>
{{- end}}
{{- end}}
	</array>
{{- end}}
{{- end}}
</dict>
</plist>
`
