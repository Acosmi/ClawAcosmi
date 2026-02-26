// Package middleware — 租户 ID 上下文传播辅助工具。
// 提供从 context.Context 中提取 tenant_id 的标准方法，
// 供 service 层统一调用，替代硬编码 "default"。
package middleware

import "context"

// tenantIDKey 是存放 tenant_id 的 context key 类型。
type tenantIDKey struct{}

// ContextKeyTenantID 导出的 context key，用于在中间件中设置 tenant_id。
var ContextKeyTenantID = tenantIDKey{}

// WithTenantID 将 tenant_id 写入 context。
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenantID, tenantID)
}

// TenantFromCtx 从 context 中提取 tenant_id。
// 如果 context 中没有 tenant_id，返回 "default" 作为兜底值，保持向后兼容。
func TenantFromCtx(ctx context.Context) string {
	if tid, ok := ctx.Value(ContextKeyTenantID).(string); ok && tid != "" {
		return tid
	}
	return "default"
}
