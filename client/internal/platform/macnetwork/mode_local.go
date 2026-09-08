//go:build jeemi_local_test

package macnetwork

const LocalTesting = true
const ServiceName = "com.jeemi.Jeemi.NetworkHelper.LocalTest"
const LocalTrustDirectory = "/Library/Application Support/JeemiNetworkHelperLocalTest"
const StateDirectory = LocalTrustDirectory + "/runtime"
