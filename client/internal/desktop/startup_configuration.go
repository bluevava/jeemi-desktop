package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"jeemi/internal/application"
	"jeemi/internal/configfile"
	"jeemi/internal/platform/dialogs"
)

type startupDialogLabels struct {
	Title, Invalid, InvalidOffset, ConfirmMessage, Confirm, Cancel string
	ErrorTitle, ReadError, DeleteError, OtherError                 string
	Quit                                                           string
}

// InitializeClient is called by the renderer's startup gate, after Wails has
// acquired its single-instance lock and created the native window on every OS.
// It runs once even during StrictMode remounts or a later WebView reload.
func (a *App) InitializeClient(language string) error {
	if a.hostReady == nil {
		return errors.New("client startup is unavailable")
	}
	<-a.hostReady
	a.initializeOnce.Do(func() {
		ctx := a.context()
		if ctx == nil || a.quitRequested.Load() {
			a.initializeError = errors.New("client startup was cancelled")
			return
		}
		var labelsByLanguage map[string]startupDialogLabels
		if err := json.Unmarshal(a.startupLabels, &labelsByLanguage); err != nil {
			a.initializeError = errors.New("client startup labels are unavailable")
			a.requestQuit(ctx)
			return
		}
		labels := labelsByLanguage["zh-CN"]
		if language == "en-US" {
			labels = labelsByLanguage[language]
		}
		var service *application.Service
		err := configfile.Recover(ctx, a.dataDirectory, func() error {
			if a.quitRequested.Load() {
				return context.Canceled
			}
			if err := application.CheckStartupConfiguration(a.dataDirectory); err != nil {
				return err
			}
			var err error
			service, err = application.NewService(a.dataDirectory)
			return err
		}, func(failure *configfile.Failure) (bool, error) {
			message := startupFailureMessage(labels, failure) + "\n\n" + labels.ConfirmMessage
			confirmed, err := dialogs.Confirm(ctx, dialogs.Confirmation{
				Title: labels.Title, Message: message, Confirm: labels.Confirm, Cancel: labels.Cancel,
			})
			return confirmed && !a.quitRequested.Load(), err
		})
		if err != nil {
			a.initializeError = errors.New("client startup did not complete")
			if !errors.Is(err, configfile.ErrDeclined) && !errors.Is(err, context.Canceled) && !a.quitRequested.Load() {
				message := labels.OtherError
				var failure *configfile.Failure
				if errors.As(err, &failure) {
					message = startupFailureMessage(labels, failure)
				}
				_ = dialogs.ShowError(ctx, labels.ErrorTitle, message, labels.Quit)
			}
			a.requestQuit(ctx)
			return
		}
		// Publish the service only once. Exit cleanup must wait for initialization
		// to finish so it cannot miss or race startup network recovery.
		a.serviceMu.Lock()
		defer a.serviceMu.Unlock()
		if a.quitRequested.Load() || ctx.Err() != nil {
			a.initializeError = errors.New("client startup was cancelled")
			return
		}
		a.service = service
		service.Startup(ctx)
		if !a.quitRequested.Load() {
			service.AcknowledgeJeemiRestart()
			a.startTray(ctx)
		}
	})
	return a.initializeError
}

func (a *App) currentService() *application.Service {
	a.serviceMu.Lock()
	defer a.serviceMu.Unlock()
	return a.service
}

func startupFailureMessage(labels startupDialogLabels, failure *configfile.Failure) string {
	template := labels.Invalid
	if failure.Reason == "read" {
		template = labels.ReadError
	} else if failure.Reason == "delete" {
		template = labels.DeleteError
	} else if failure.Offset > 0 {
		template = labels.InvalidOffset
	}
	return strings.NewReplacer("{{file}}", failure.Path, "{{offset}}", strconv.FormatInt(failure.Offset, 10)).Replace(template)
}
