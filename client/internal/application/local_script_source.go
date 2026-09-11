package application

import (
	"context"
	"fmt"
	"strings"

	"jeemi/internal/localscript"
)

// Download outside the configuration locks; the captured revision is checked
// again by Preview/Save before any validated candidate can replace local files.
func (s *Service) prepareLocalScriptInput(ctx context.Context, input localscript.SaveInput, forceDownload bool) (localscript.SaveInput, error) {
	var current localscript.Script
	if input.ID != "" {
		var err error
		current, err = s.localScripts.Get(input.ID)
		if err != nil {
			return input, err
		}
		if input.ExpectedRevision > 0 && input.ExpectedRevision != current.Revision {
			return input, fmt.Errorf("local script changed; reopen it before saving")
		}
		input.ExpectedRevision = current.Revision
	}
	if input.SourceURL == "" {
		value := strings.TrimSpace(input.Contents)
		lower := strings.ToLower(value)
		if (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) && !strings.ContainsAny(value, "\r\n\t ") {
			input.SourceURL = value
		}
	}
	if input.SourceURL == "" {
		return input, nil
	}
	address, err := localscript.NormalizeSourceURL(input.SourceURL)
	if err != nil {
		return input, err
	}
	input.SourceURL = address
	if !forceDownload && current.SourceURL == address {
		input.Contents = current.Contents
		return input, nil
	}
	fetcher := s.scriptFetcher
	if fetcher == nil {
		fetcher = &localscript.HTTPFetcher{}
	}
	contents, err := fetcher.Fetch(ctx, address)
	if err != nil {
		return input, err
	}
	input.Contents = contents
	return input, ctx.Err()
}

func (s *Service) RefreshLocalScript(id string) (localscript.State, error) {
	current, err := s.localScripts.Get(id)
	if err != nil {
		return localscript.State{}, err
	}
	if current.SourceURL == "" {
		return localscript.State{}, fmt.Errorf("local script has no download URL")
	}
	_, err = s.saveLocalScript(localscript.SaveInput{
		ID: current.ID, Name: current.Name, Description: current.Description,
		SourceURL: current.SourceURL, ExpectedRevision: current.Revision,
	}, true)
	if err != nil {
		return localscript.State{}, err
	}
	return s.localScripts.State()
}
