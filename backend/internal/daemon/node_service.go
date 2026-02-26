package daemon

// nodeServiceWrapper 包装 GatewayService，注入 Node 服务环境变量
// 对应 TS: node-service.ts resolveNodeService
type nodeServiceWrapper struct {
	base GatewayService
}

// ResolveNodeService 创建 Node 服务实例
// 在基础 GatewayService 上包装一层，自动注入 Node 服务标识
// 对应 TS: node-service.ts resolveNodeService
func ResolveNodeService() GatewayService {
	return &nodeServiceWrapper{base: ResolveGatewayService()}
}

func (w *nodeServiceWrapper) Label() string         { return w.base.Label() }
func (w *nodeServiceWrapper) LoadedText() string    { return w.base.LoadedText() }
func (w *nodeServiceWrapper) NotLoadedText() string { return w.base.NotLoadedText() }

func (w *nodeServiceWrapper) Install(args GatewayServiceInstallArgs) error {
	args.Env = WithNodeServiceEnv(args.Env)
	args.Environment = WithNodeServiceEnv(args.Environment)
	return w.base.Install(args)
}

func (w *nodeServiceWrapper) Uninstall(env map[string]string) error {
	return w.base.Uninstall(WithNodeServiceEnv(env))
}

func (w *nodeServiceWrapper) Stop(env map[string]string) error {
	return w.base.Stop(WithNodeServiceEnv(env))
}

func (w *nodeServiceWrapper) Restart(env map[string]string) error {
	return w.base.Restart(WithNodeServiceEnv(env))
}

func (w *nodeServiceWrapper) IsLoaded(env map[string]string) (bool, error) {
	return w.base.IsLoaded(WithNodeServiceEnv(env))
}

func (w *nodeServiceWrapper) ReadCommand(env map[string]string) (*GatewayServiceCommand, error) {
	return w.base.ReadCommand(WithNodeServiceEnv(env))
}

func (w *nodeServiceWrapper) ReadRuntime(env map[string]string) (GatewayServiceRuntime, error) {
	return w.base.ReadRuntime(WithNodeServiceEnv(env))
}
