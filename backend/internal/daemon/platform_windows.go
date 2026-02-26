//go:build windows

package daemon

func newPlatformService() GatewayService {
	return newSchtasksService()
}
