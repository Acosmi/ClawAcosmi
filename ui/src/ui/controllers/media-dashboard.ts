// controllers/media-dashboard.ts — 媒体运营仪表盘控制器
// 管理热点话题、草稿列表的数据加载逻辑。

import type { AppViewState } from "../app-view-state.ts";

// ---------- 类型 ----------

export interface TrendingTopic {
  title: string;
  source: string;
  url?: string;
  heat_score: number;
  category?: string;
  fetched_at: string;
}

export interface DraftEntry {
  id: string;
  title: string;
  body: string;
  platform: string;
  style: string;
  status: string;
  created_at: string;
  updated_at: string;
  tags?: string[];
  images?: string[];
}

// ---------- 加载函数 ----------

export async function loadTrendingSources(state: AppViewState): Promise<void> {
  if (!state.client || !state.connected) return;
  try {
    const res = await state.client.request<{ sources: string[] }>("media.trending.sources");
    if (res?.sources) {
      state.mediaTrendingSources = res.sources;
    }
  } catch {
    // 忽略加载失败
  }
}

export async function loadTrendingTopics(
  state: AppViewState,
  source?: string,
  category?: string,
): Promise<void> {
  if (!state.client || !state.connected) return;
  state.mediaTrendingLoading = true;
  try {
    const params: Record<string, unknown> = { limit: 30 };
    if (source) params.source = source;
    if (category) params.category = category;

    const res = await state.client.request<{
      topics: TrendingTopic[];
      count: number;
      errors?: Array<{ source: string; error: string }>;
    }>("media.trending.fetch", params);

    if (res) {
      state.mediaTrendingTopics = res.topics || [];
    }
  } catch {
    state.mediaTrendingTopics = [];
  } finally {
    state.mediaTrendingLoading = false;
  }
}

export async function loadDraftsList(state: AppViewState, platform?: string): Promise<void> {
  if (!state.client || !state.connected) return;
  state.mediaDraftsLoading = true;
  try {
    const params: Record<string, unknown> = {};
    if (platform) params.platform = platform;

    const res = await state.client.request<{
      drafts: DraftEntry[];
      count: number;
    }>("media.drafts.list", params);

    if (res) {
      state.mediaDrafts = res.drafts || [];
    }
  } catch {
    state.mediaDrafts = [];
  } finally {
    state.mediaDraftsLoading = false;
  }
}

export async function deleteDraft(state: AppViewState, id: string): Promise<boolean> {
  if (!state.client || !state.connected) return false;
  try {
    await state.client.request("media.drafts.delete", { id });
    // 刷新列表
    await loadDraftsList(state);
    return true;
  } catch {
    return false;
  }
}

// ---------- 发布历史 ----------

export interface PublishRecord {
  id: string;
  draft_id: string;
  title: string;
  platform: string;
  post_id?: string;
  url?: string;
  status: string;
  published_at: string;
}

export async function loadPublishHistory(state: AppViewState): Promise<void> {
  if (!state.client || !state.connected) return;
  state.mediaPublishLoading = true;
  try {
    const res = await state.client.request<{
      records: PublishRecord[];
      count: number;
    }>("media.publish.list");

    if (res) {
      state.mediaPublishRecords = res.records || [];
    }
  } catch {
    state.mediaPublishRecords = [];
  } finally {
    state.mediaPublishLoading = false;
  }
}

export async function loadMediaDashboard(state: AppViewState): Promise<void> {
  await Promise.all([
    loadTrendingSources(state),
    loadDraftsList(state),
    loadPublishHistory(state),
  ]);
}
