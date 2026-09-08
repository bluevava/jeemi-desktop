package platform

import "runtime"

type Info struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func Current() Info {
	return Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}
