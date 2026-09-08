package application

import (
	"time"

	"jeemi/internal/appupdate"
)

func (s *Service) CheckJeemiUpdates() (appupdate.Result, error) {
	ctx, cancel := s.operationContext(20 * time.Second)
	defer cancel()
	return s.jeemiUpdater.Check(ctx)
}

func (s *Service) JeemiUpdateState() appupdate.State  { return s.jeemiUpdater.State() }
func (s *Service) CancelJeemiUpdate()                 { s.jeemiUpdater.Cancel() }
func (s *Service) AcknowledgeJeemiRestart()           { s.jeemiUpdater.AcknowledgeRestart() }
func (s *Service) DismissJeemiUpdateResult(id string) { s.jeemiUpdater.DismissResult(id) }

func (s *Service) InstallJeemiUpdate(version string) error {
	ctx, cancel := s.operationContext(11 * time.Minute)
	defer cancel()
	if err := s.jeemiUpdater.Prepare(ctx, version); err != nil {
		return err
	}
	// Block new explicit starts while the normal stop path restores networking.
	s.mu.Lock()
	s.shuttingDown = true
	s.mu.Unlock()
	if _, err := s.StopProxy(); err != nil {
		s.mu.Lock()
		s.shuttingDown = false
		s.mu.Unlock()
		s.jeemiUpdater.Abort(appupdate.ErrStop)
		return appupdate.ErrStop
	}
	if err := s.jeemiUpdater.Commit(); err != nil {
		s.mu.Lock()
		s.shuttingDown = false
		s.mu.Unlock()
		s.jeemiUpdater.Abort(err)
		return err
	}
	return nil
}
