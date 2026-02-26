package daemon

import "runtime"

// ResolveGatewayService 根据当前平台返回对应的 GatewayService 实现
// 对应 TS: service.ts resolveGatewayService
func ResolveGatewayService() GatewayService {
	switch runtime.GOOS {
	case "darwin":
		return newPlatformService()
	case "linux":
		return newPlatformService()
	case "windows":
		return newPlatformService()
	default:
		return &unsupportedService{platform: runtime.GOOS}
	}
}

// unsupportedService 不支持的平台回退实现
type unsupportedService struct {
	platform string
}

func (s *unsupportedService) Label() string         { return "unsupported (" + s.platform + ")" }
func (s *unsupportedService) LoadedText() string    { return "unknown" }
func (s *unsupportedService) NotLoadedText() string { return "unknown" }

func (s *unsupportedService) Install(_ GatewayServiceInstallArgs) error {
	return &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) Uninstall(_ map[string]string) error {
	return &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) Stop(_ map[string]string) error {
	return &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) Restart(_ map[string]string) error {
	return &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) IsLoaded(_ map[string]string) (bool, error) {
	return false, &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) ReadCommand(_ map[string]string) (*GatewayServiceCommand, error) {
	return nil, &ErrUnsupportedPlatform{Platform: s.platform}
}

func (s *unsupportedService) ReadRuntime(_ map[string]string) (GatewayServiceRuntime, error) {
	return GatewayServiceRuntime{}, &ErrUnsupportedPlatform{Platform: s.platform}
}

// ErrUnsupportedPlatform 表示不支持的平台错误
type ErrUnsupportedPlatform struct {
	Platform string
}

func (e *ErrUnsupportedPlatform) Error() string {
	return "不支持的平台: " + e.Platform
}
