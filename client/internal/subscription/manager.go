package subscription

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"strings"

	platformscreen "jeemi/internal/platform/screen"
)

type ManagerOptions struct {
	DataDirectory string
	Store         *Store
	Fetcher       Fetcher
	// RefreshFallback is consulted only after a failed refresh fetch. A nil
	// fetcher means that the alternate route is currently unavailable.
	RefreshFallback func() Fetcher
	IconDetector    IconDetector
	QRDecoder       QRDecoder
	Screen          platformscreen.Capturer
	ValidateImport  func(context.Context, []byte) error
}

type Manager struct {
	store           *Store
	fetcher         Fetcher
	refreshFallback func() Fetcher
	icons           IconDetector
	qrDecoder       QRDecoder
	screen          platformscreen.Capturer
	validateImport  func(context.Context, []byte) error
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
	return &Manager{store: store, fetcher: fetcher, refreshFallback: options.RefreshFallback, icons: icons, qrDecoder: decoder, screen: screenCapturer, validateImport: options.ValidateImport}, nil
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
	interval, err := normalizeAutoUpdateInterval(SourceURL, input.AutoUpdateIntervalMinutes)
	if err != nil {
		return State{}, err
	}
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
	if m.validateImport != nil {
		if err := m.validateImport(ctx, fetched.Contents); err != nil {
			return State{}, err
		}
	}
	if icon == "" {
		iconKind, icon = m.detectIconBestEffort(ctx, input.SourceURL)
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	_, err = m.store.Create(createInput{
		AutoUpdateIntervalMinutes: interval,
		Name:                      name,
		Description:               input.Description,
		SourceKind:                SourceURL,
		ImportMethod:              method,
		SourceURL:                 input.SourceURL,
		IconKind:                  iconKind,
		Icon:                      icon,
		Format:                    fetched.Format,
		Contents:                  fetched.Contents,
		RemoteProfile:             fetched.RemoteProfile,
	})
	if err != nil {
		return State{}, err
	}
	return m.store.State()
}

func (m *Manager) ImportFile(path string) (State, error) {
	path, err := NormalizeFileSourcePath(path)
	if err != nil {
		return State{}, err
	}
	fetched, err := readFileSource(context.Background(), path)
	if err != nil {
		return State{}, err
	}
	base := filepath.Base(path)
	if m.validateImport != nil {
		if err := m.validateImport(context.Background(), fetched.Contents); err != nil {
			return State{}, err
		}
	}
	name := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" {
		name = "Subscription"
	}
	_, err = m.store.Create(createInput{
		Name:             name,
		SourceKind:       SourceFile,
		ImportMethod:     ImportFile,
		OriginalFileName: base,
		SourceFilePath:   path,
		Format:           fetched.Format,
		Contents:         fetched.Contents,
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
	Before         Detail
	Input          UpdateInput
	Fetched        FetchedContent
	SourceFilePath string
}

func (m *Manager) PrepareRefresh(ctx context.Context, id string) (RefreshCandidate, error) {
	detail, err := m.store.Get(id)
	if err != nil {
		return RefreshCandidate{}, err
	}
	var fetched FetchedContent
	if detail.SourceKind == SourceFile {
		fetched, err = readFileSource(ctx, detail.SourceFilePath)
	} else {
		fetched, err = m.fetchRefresh(ctx, detail.SourceURL)
	}
	if err != nil {
		return RefreshCandidate{}, err
	}
	if err := validateContents(fetched.Format, fetched.Contents); err != nil {
		return RefreshCandidate{}, err
	}
	iconKind, icon := detail.IconKind, detail.Icon
	if icon == "" && detail.SourceKind == SourceURL {
		iconKind, icon = m.detectIconBestEffort(ctx, detail.SourceURL)
	}
	if err := ctx.Err(); err != nil {
		return RefreshCandidate{}, err
	}
	return RefreshCandidate{Before: detail, Fetched: fetched, SourceFilePath: detail.SourceFilePath, Input: UpdateInput{
		AutoUpdateIntervalMinutes: detail.AutoUpdateIntervalMinutes,
		ID:                        detail.ID, Name: detail.Name, Description: detail.Description, SourceURL: detail.SourceURL, IconKind: iconKind, Icon: icon,
	}}, nil
}

func (m *Manager) CommitRefresh(candidate RefreshCandidate, detach bool) (State, error) {
	if _, err := m.store.updateFromSourceCandidate(candidate.Input, candidate.Fetched, candidate.SourceFilePath, &candidate.Before, detach); err != nil {
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
	return m.updateValidated(ctx, input, "", validate)
}

// UpdateFileValidated is only called after the application resolves a native
// picker token. An arbitrary path is never accepted from a Wails update DTO.
func (m *Manager) UpdateFileValidated(ctx context.Context, input UpdateInput, path string, validate func(Summary, []byte) error) (State, error) {
	path, err := NormalizeFileSourcePath(path)
	if err != nil {
		return State{}, err
	}
	return m.updateValidated(ctx, input, path, validate)
}

func (m *Manager) updateValidated(ctx context.Context, input UpdateInput, path string, validate func(Summary, []byte) error) (State, error) {
	detail, err := m.store.Get(input.ID)
	if err != nil {
		return State{}, err
	}
	if input.SourceFileToken != "" || path != "" && detail.SourceKind != SourceFile {
		return State{}, fmt.Errorf("subscription file selection is invalid")
	}
	input.AutoUpdateIntervalMinutes, err = normalizeAutoUpdateInterval(detail.SourceKind, input.AutoUpdateIntervalMinutes)
	if err != nil {
		return State{}, err
	}
	if detail.SourceKind == SourceFile {
		if path != "" {
			fetched, err := readFileSource(ctx, path)
			if err != nil {
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
			if _, err := m.store.updateFromSourceCandidate(input, fetched, path, &detail, false); err != nil {
				return State{}, err
			}
			return m.store.State()
		}
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
	case ".txt", ".conf":
		return FormatText, nil
	default:
		return "", fmt.Errorf("subscription file must use .yaml, .json, .txt, or .conf")
	}
}
