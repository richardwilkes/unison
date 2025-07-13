package packager

import (
	"archive/zip"
	"image"
	"image/png"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func prepareBinary(cfg *Config) error {
	rs := &winres.ResourceSet{}
	rs.SetManifest(winres.AppManifest{
		Description:    cfg.Description,
		Compatibility:  winres.Win10AndAbove,
		ExecutionLevel: winres.AsInvoker,
		DPIAwareness:   winres.DPIAware,
	})
	if err := addWindowsIcon(cfg, rs); err != nil {
		return err
	}
	if err := addWindowsVersion(cfg, rs); err != nil {
		return err
	}
	f, err := os.Create("rsrc_windows_amd64.syso")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if err = rs.WriteObject(f, winres.ArchAMD64); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func addWindowsIcon(cfg *Config, rs *winres.ResourceSet) error {
	appImg, err := loadPNG(cfg.AppIcon)
	if err != nil {
		return err
	}
	var winIcon *winres.Icon
	if winIcon, err = winres.NewIconFromResizedImage(appImg, nil); err != nil {
		return errs.Wrap(err)
	}
	if err = rs.SetIconTranslation(winres.Name("APP"), 0, winIcon); err != nil {
		return errs.Wrap(err)
	}
	for _, fi := range cfg.FileInfo {
		if fi.Rank != "Owner" {
			continue
		}
		var docImg image.Image
		docImg, err = loadPNG(fi.Icon)
		if err != nil {
			return err
		}
		if winIcon, err = winres.NewIconFromResizedImage(docImg, nil); err != nil {
			return errs.Wrap(err)
		}
		if err = rs.SetIconTranslation(winres.Name(fi.Extensions[0]), 0, winIcon); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errs.Wrap(closeErr)
		}
	}()
	return png.Decode(f)
}

func addWindowsVersion(cfg *Config, rs *winres.ResourceSet) error {
	var vi version.Info
	vi.SetFileVersion(cfg.version)
	vi.SetProductVersion(cfg.version)
	cmdName := cfg.ExecutableName + ".exe"
	shortAppVersion := cfg.shortAppVersion()
	if err := vi.Set(version.LangDefault, version.CompanyName, cfg.CopyrightHolder); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.FileDescription, cfg.Description); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.FileVersion, shortAppVersion); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.InternalName, cmdName); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.LegalCopyright, cfg.copyright()); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.LegalTrademarks, cfg.Trademarks); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.OriginalFilename, cmdName); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.ProductName, cfg.FullName); err != nil {
		return errs.Wrap(err)
	}
	if err := vi.Set(version.LangDefault, version.ProductVersion, shortAppVersion); err != nil {
		return errs.Wrap(err)
	}
	rs.SetVersionInfo(vi)
	return nil
}

func generateDistribution(cfg *Config) (err error) {
	dstPath := cfg.ExecutableName + "-" + cfg.version + "-windows-" + runtime.GOARCH + ".zip"
	if xos.FileExists(dstPath) {
		if err = os.Remove(dstPath); err != nil {
			return errs.Wrap(err)
		}
	}
	exeName := cfg.ExecutableName + ".exe"
	var in, out *os.File
	if in, err = os.Open(exeName); err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if out, err = os.Create(dstPath); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	zw := zip.NewWriter(out)
	defer func() {
		if closeErr := zw.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	var fw io.Writer
	hdr := &zip.FileHeader{
		Name:     exeName,
		Method:   zip.Deflate,
		Modified: time.Now(),
	}
	hdr.SetMode(0o755)
	if fw, err = zw.CreateHeader(hdr); err != nil {
		err = errs.Wrap(err)
		return err
	}
	if _, err = io.Copy(fw, in); err != nil {
		err = errs.Wrap(err)
		return err
	}
	return nil
}
