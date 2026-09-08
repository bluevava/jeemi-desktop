package helperstate

import (
	"errors"
	"reflect"
	"testing"
)

func TestReplacementFailurePreservesRepairableInstallation(t *testing.T) {
	for _, tt := range []struct {
		fail string
		want []string
	}{{"prepare", []string{"prepare"}}, {"stop", []string{"prepare", "stop"}}, {"promote", []string{"prepare", "stop", "promote", "rollback"}}, {"verify", []string{"prepare", "stop", "promote", "verify", "rollback"}}, {"", []string{"prepare", "stop", "promote", "verify"}}} {
		t.Run(tt.fail, func(t *testing.T) {
			var calls []string
			step := func(name string) func() error {
				return func() error {
					calls = append(calls, name)
					if name == tt.fail {
						return errors.New(name)
					}
					return nil
				}
			}
			err := Replace(step("prepare"), step("stop"), step("promote"), step("verify"), step("rollback"))
			if (err == nil) != (tt.fail == "") || !reflect.DeepEqual(calls, tt.want) {
				t.Fatal(calls, err)
			}
		})
	}
}
