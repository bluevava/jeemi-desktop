package desktop

import "jeemi/internal/iconcache"

func (a *App) GetSelectorIcon(id, address string) (string, error) {
	if a.service == nil {
		return "", iconcache.ErrUnavailable
	}
	return a.service.SelectorIcon(id, address)
}

func (a *App) CancelSelectorIcon(id string) {
	if a.service != nil {
		a.service.CancelSelectorIcon(id)
	}
}
