package desktop

import (
	"jeemi/internal/localscript"
)

func (a *App) GetLocalScriptState() (localscript.State, error) {
	return a.service.LocalScriptState()
}

func (a *App) GetLocalScript(id string) (localscript.Script, error) {
	return a.service.LocalScript(id)
}

func (a *App) TestLocalScript(input localscript.TestInput) (localscript.TestResult, error) {
	return a.service.TestLocalScript(input)
}

func (a *App) SaveLocalScript(input localscript.SaveInput) (localscript.Script, error) {
	return a.service.SaveLocalScript(input)
}

func (a *App) DeleteLocalScript(id string) (localscript.State, error) {
	return a.service.DeleteLocalScript(id)
}
