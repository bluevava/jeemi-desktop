package application

import (
	"errors"
	platformfiles "jeemi/internal/platform/files"
	"jeemi/internal/uidiagnostics"
)

func (s *Service) ReportUIFailure(input uidiagnostics.Failure) error {
	return s.uiDiagnostics.Report(input)
}

func (s *Service) OpenUIDiagnosticsDirectory() error {
	directory, err := s.uiDiagnostics.Directory()
	if err != nil {
		return err
	}
	if err := platformfiles.OpenDirectory(directory); err != nil {
		return errors.New("could not open UI diagnostics directory")
	}
	return nil
}
