package packager

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/errs"
)

// Package performs the platform-specific packaging for a Unison application.
func Package(cfg *Config, version string, createDist bool) error {
	cfg.prepare(version)
	if err := prepareBinary(cfg); err != nil {
		return err
	}
	if createDist {
		return generateDistribution(cfg)
	}
	return nil
}

func copyFile(from, to string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return errs.Wrap(err)
	}
	f, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	var s *os.File
	if s, err = os.Open(from); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := s.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if _, err = io.Copy(f, s); err != nil {
		err = errs.Wrap(err)
		return err
	}
	return nil
}
