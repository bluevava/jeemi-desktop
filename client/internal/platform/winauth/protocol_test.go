package winauth

import "testing"

func TestHelperProtocolRejectsGenericPrivilegeRequests(t *testing.T) {
	for _, data := range []string{
		`{"operation":"memory","pid":4}`,
		`{"operation":"memory","target":{}}`,
		`{"operation":"memory","preferences":{}}`,
		`{"operation":"status","webViewMemory":true}`,
		`{"operation":"exec","command":"cmd.exe"}`,
		`{"operation":"prepare","coreSize":1024}`,
		`{"operation":"check","coreSize":1024}`,
		`{"operation":"start","target":{"version":"v1.2.3","sha256":"fake","size":100}}`,
		`{"operation":"remove","path":"C:\\Windows"}`,
		`{"operation":"status","preferences":{}}`,
		`{"operation":"stage","configuration":"../config.yaml","sourceRoot":"C:/data","snapshotSize":10}`,
		`{"operation":"stage","configuration":"generations/one/config.yaml","sourceRoot":"C:/data","snapshotSize":99999999999}`,
		`{"operation":"status"} {"operation":"remove"}`,
	} {
		if _, err := decodeRequest([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	if _, err := decodeRequest([]byte(`{"operation":"status"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRequest([]byte(`{"operation":"memory","webViewMemory":true}`)); err != nil {
		t.Fatal(err)
	}
}
