//go:build linux

package daemon

func newPlatformService() GatewayService {
	return newSystemdService()
}
