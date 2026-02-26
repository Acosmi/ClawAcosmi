package cron

// ============================================================================
// CronService — 公共 API 入口
// 对应 TS: cron/service.ts (49L)
// ============================================================================

// CronService 定时任务服务
type CronService struct {
	state *CronServiceState
}

// NewCronService 创建 CronService
func NewCronService(deps CronServiceDeps) *CronService {
	return &CronService{
		state: CreateCronServiceState(deps),
	}
}

// Start 启动服务
func (s *CronService) Start() error {
	return Start(s.state)
}

// Stop 停止服务
func (s *CronService) Stop() {
	Stop(s.state)
}

// Status 查询状态
func (s *CronService) Status() CronStatusResult {
	return Status(s.state)
}

// List 列出 jobs
func (s *CronService) List(includeDisabled bool) ([]CronJob, error) {
	return List(s.state, includeDisabled)
}

// Add 添加 job
func (s *CronService) Add(input CronJobCreate) (*CronAddResult, error) {
	return Add(s.state, input)
}

// Update 更新 job
func (s *CronService) Update(id string, patch CronJobPatch) (*CronOpResult, error) {
	return Update(s.state, id, patch)
}

// Remove 删除 job
func (s *CronService) Remove(id string) (*CronOpResult, error) {
	return Remove(s.state, id)
}

// Run 手动运行 job
func (s *CronService) Run(id string, mode string) (*CronRunResult, error) {
	return Run(s.state, id, mode)
}

// Wake 唤醒
func (s *CronService) Wake(mode string, text string) *CronOpResult {
	return WakeNow(s.state, mode, text)
}
