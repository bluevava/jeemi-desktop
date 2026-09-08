package subscription

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	platformscreen "jeemi/internal/platform/screen"
)

type ManagerOptions struct {
	DataDirectory string
	Store         *Store
	Fetcher       Fetcher
	IconDetector  IconDetector
	QRDecoder     QRDecoder
	Screen        platformscreen.Capturer
}

type Manager struct {
	store     *Store
	fetcher   Fetcher
	icons     IconDetector
	qrDecoder QRDecoder
	screen    platformscreen.Capturer
}

func NewManager(options ManagerOptions) (*Manager, error) {
	store := options.Store
	if store == nil {
		var err error
		store, err = NewStore(StoreOptions{DataDirectory: options.DataDirectory})
		if err != nil {
			return nil, err
		}
	}
	fetcher := options.Fetcher
	customFetcher := fetcher != nil
	if fetcher == nil {
		fetcher = NewHTTPFetcher()
	}
	icons := options.IconDetector
	if icons == nil {
		if customFetcher {
			icons = disabledIconDetector{}
		} else {
			icons = NewHTTPIconDetector()
		}
	}
	decoder := options.QRDecoder
	if decoder == nil {
		decoder = ZXingQRDecoder{}
	}
	screenCapturer := options.Screen
	if screenCapturer == nil {
		screenCapturer = platformscreen.DesktopCapturer{}
	}
	return &Manager{store: store, fetcher: fetcher, icons: icons, qrDecoder: decoder, screen: screenCapturer}, nil
}

func (m *Manager) State() (State, error) {
	return m.store.State()
}

func (m *Manager) Directory() string {
	return m.store.Directory()
}

func (m *Manager) Get(id string) (Detail, error) {
	return m.store.Get(id)
}

func (m *Manager) ImportURL(ctx context.Context, input ImportURLInput) (State, error) {
	method := input.ImportMethod
	if method == "" {
		method = ImportURL
	}
	if method != ImportURL && method != ImportQRImage && method != ImportQRScreen {
		return State{}, fmt.Errorf("subscription URL import method is invalid")
	}
	iconKind, icon, err := validateIcon(input.IconKind, input.Icon)
	if err != nil {
		return State{}, err
	}
	fetched, err := m.fetcher.Fetch(ctx, input.SourceURL)
	if err != nil {
		return State{}, err
	}
	name := chooseImportedName(input.Name, fetched.SuggestedName, input.SourceURL)
	if icon == "" {
		iconKind, icon = m.detectIconBestEffort(ctx, input.SourceURL)
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	_, err = m.store.Create(createInput{
		Name:          name,
		Description:   input.Description,
		SourceKind:    SourceURL,
		ImportMethod:  method,
		SourceURL:     input.SourceURL,
		IconKind:      iconKind,
		Icon:          icon,
		Format:        fetched.Format,
		Contents:      fetched.Contents,
		RemoteProfile: fetched.RemoteProfile,
	})
	if err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) ImportFile(path string) (State, error) {
	format, err := formatForImportedFile(path)
	if err != nil {
		return State{}, err
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxSubscriptionBytes {
		return State{}, fmt.Errorf("subscription file is unavailable or too large")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read subscription file")
	}
	base := filepath.Base(path)
	name := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" {
		name = "Subscription"
	}
	_, err = m.store.Create(createInput{
		Name:             name,
		SourceKind:       SourceFile,
		ImportMethod:     ImportFile,
		OriginalFileName: base,
		Format:           format,
		Contents:         contents,
	})
	if err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) ImportQRImage(ctx context.Context, path string) (State, error) {
	imageValue, err := loadQRCodeImage(path)
	if err != nil {
		return State{}, err
	}
	return m.importQRImages(ctx, []image.Image{imageValue}, ImportQRImage)
}

func (m *Manager) ImportQRScreen(ctx context.Context) (State, error) {
	images, err := m.CaptureQRScreen(ctx)
	if err != nil {
		return State{}, err
	}
	return m.ImportQRScreenImages(ctx, images)
}

func (m *Manager) CaptureQRScreen(ctx context.Context) ([]image.Image, error) {
	return m.screen.CaptureAll(ctx)
}

func (m *Manager) ImportQRScreenImages(ctx context.Context, images []image.Image) (State, error) {
	return m.importQRImages(ctx, images, ImportQRScreen)
}

func (m *Manager) ImportRecognizedURL(ctx context.Context, sourceURL string, method ImportMethod) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	parsed, err := validateRemoteURL(sourceURL)
	if err != nil {
		return State{}, err
	}
	return m.ImportURL(ctx, ImportURLInput{
		Name:         "",
		SourceURL:    parsed.String(),
		ImportMethod: method,
	})
}

// RefreshCandidate contains sensitive bytes only in Go memory. It is never a UI DTO.
type RefreshCandidate struct {
	Before  Detail
	Input   UpdateInput
	Fetched FetchedContent
}

func (m *Manager) PrepareRefresh(ctx context.Context, id string) (RefreshCandidate, error) {
	detail, err := m.store.Get(id)
	if err != nil {
		return RefreshCandidate{}, err
	}
	if detail.SourceKind != SourceURL {
		return RefreshCandidate{}, fmt.Errorf("file subscriptions cannot be refreshed from a remote source")
	}
	fetched, err := m.fetcher.Fetch(ctx, detail.SourceURL)
	if err != nil {
		return RefreshCandidate{}, err
	}
	if err := validateContents(fetched.Format, fetched.Contents); err != nil {
		return RefreshCandidate{}, err
	}
	iconKind, icon := detail.IconKind, detail.Icon
	if icon == "" {
		iconKind, icon = m.detectIconBestEffort(ctx, detail.SourceURL)
	}
	return RefreshCandidate{Before: detail, Fetched: fetched, Input: UpdateInput{
		ID: detail.ID, Name: detail.Name, Description: detail.Description, SourceURL: detail.SourceURL, IconKind: iconKind, Icon: icon,
	}}, nil
}

func (m *Manager) CommitRefresh(candidate RefreshCandidate, detach bool) (State, error) {
	if _, err := m.store.updateFromURLCandidate(candidate.Input, candidate.Fetched, &candidate.Before, detach); err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) Refresh(ctx context.Context, id string) (State, error) {
	candidate, err := m.PrepareRefresh(ctx, id)
	if err != nil {
		return State{}, err
	}
	return m.CommitRefresh(candidate, false)
}

func (m *Manager) Update(ctx context.Context, input UpdateInput) (State, error) {
	return m.UpdateValidated(ctx, input, nil)
}

func (m *Manager) UpdateValidated(ctx context.Context, input UpdateInput, validate func(Summary, []byte) error) (State, error) {
	detail, err := m.store.Get(input.ID)
	if err != nil {
		return State{}, err
	}
	if detail.SourceKind == SourceFile {
		if _, err := m.store.UpdateMetadata(input); err != nil {
			return State{}, err
		}
		return m.store.State()
	}
	input.IconKind, input.Icon, err = validateIcon(input.IconKind, input.Icon)
	if err != nil {
		return State{}, err
	}
	currentURL, err := validateRemoteURL(detail.SourceURL)
	if err != nil {
		return State{}, err
	}
	nextURL, err := validateRemoteURL(input.SourceURL)
	if err != nil {
		return State{}, err
	}
	if currentURL.String() == nextURL.String() {
		if input.Icon == "" {
			input.IconKind, input.Icon = m.detectIconBestEffort(ctx, nextURL.String())
		}
		if _, err := m.store.UpdateMetadata(input); err != nil {
			return State{}, err
		}
		return m.store.State()
	}
	fetched, err := m.fetcher.Fetch(ctx, nextURL.String())
	if err != nil {
		return State{}, err
	}
	if input.Icon == "" {
		input.IconKind, input.Icon = m.detectIconBestEffort(ctx, nextURL.String())
	}
	if err := validateContents(fetched.Format, fetched.Contents); err != nil {
		return State{}, err
	}
	if validate != nil {
		if err := validate(detail.Summary, fetched.Contents); err != nil {
			return State{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if _, err := m.store.updateFromURLCandidate(input, fetched, &detail, false); err != nil {
		return State{}, err
	}
	return m.store.State()
}

func chooseImportedName(explicitName, suggestedName, sourceURL string) string {
	if name := strings.TrimSpace(explicitName); name != "" {
		return name
	}
	if name := strings.TrimSpace(suggestedName); name != "" {
		return name
	}
	if parsed, err := validateRemoteURL(sourceURL); err == nil {
		if name := strings.TrimSpace(parsed.Hostname()); name != "" {
			return name
		}
	}
	return "Subscription"
}

func (m *Manager) Delete(id string) (State, error) {
	return m.store.Delete(id)
}

func (m *Manager) Text(id string) (TextView, error) {
	return m.store.Text(id)
}

func (m *Manager) SetLocalConfig(id, localConfigID string) (State, error) {
	if _, err := m.store.SetLocalConfig(id, localConfigID); err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) SetLocalScript(id, localScriptID string) (State, error) {
	if _, err := m.store.SetLocalScript(id, localScriptID); err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) SetRuleProviderEnabled(id, providerName string, enabled bool) (State, error) {
	if _, err := m.store.SetRuleProviderEnabled(id, providerName, enabled); err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) ReferencingLocalConfig(localConfigID string) ([]Summary, error) {
	return m.store.ReferencingLocalConfig(localConfigID)
}

func (m *Manager) ReferencingLocalScript(localScriptID string) ([]Summary, error) {
	return m.store.ReferencingLocalScript(localScriptID)
}

func (m *Manager) DetectIcon(ctx context.Context, sourceURL string) (IconDetectionResult, error) {
	icon, found, err := m.icons.Detect(ctx, sourceURL)
	if err != nil {
		return IconDetectionResult{}, err
	}
	if !found {
		return IconDetectionResult{}, nil
	}
	kind, normalized, err := validateIcon(IconURL, icon)
	if err != nil {
		return IconDetectionResult{}, fmt.Errorf("detected subscription icon is invalid")
	}
	return IconDetectionResult{Found: true, IconKind: kind, Icon: normalized}, nil
}

func (m *Manager) detectIconBestEffort(ctx context.Context, sourceURL string) (IconKind, string) {
	icon, found, err := m.icons.Detect(ctx, sourceURL)
	if err != nil || !found {
		return IconNone, ""
	}
	kind, normalized, err := validateIcon(IconURL, icon)
	if err != nil {
		return IconNone, ""
	}
	return kind, normalized
}

func (m *Manager) importQRImages(ctx context.Context, images []image.Image, method ImportMethod) (State, error) {
	sourceURL, err := m.recognizeURL(ctx, images)
	if err != nil {
		return State{}, err
	}
	return m.ImportRecognizedURL(ctx, sourceURL, method)
}

func (m *Manager) recognizeURL(ctx context.Context, images []image.Image) (string, error) {
	for _, source := range images {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		texts, err := m.qrDecoder.Decode(source)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			continue
		}
		for _, text := range texts {
			parsed, err := validateRemoteURL(text)
			if err != nil {
				continue
			}
			return parsed.String(), nil
		}
	}
	return "", ErrNoQRCode
}

func formatForImportedFile(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml":
		return FormatYAML, nil
	case ".json":
		return FormatJSON, nil
	case ".txt":
		return FormatText, nil
	default:
		return "", fmt.Errorf("subscription file must use .yaml, .json, or .txt")
	}
}
