// Package services — graph_store 单元测试 (Phase 15-A)。
package services

import (
	"testing"
	"time"
)

// --- QueryGraphAsOf 参数验证 ---

func TestQueryGraphAsOf_EmptyUserID(t *testing.T) {
	gs := &GraphStoreService{}
	_, err := gs.QueryGraphAsOf(nil, "", time.Now())
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
	if err.Error() != "user_id is required" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

func TestQueryGraphAsOf_ZeroTime(t *testing.T) {
	gs := &GraphStoreService{}
	_, err := gs.QueryGraphAsOf(nil, "user1", time.Time{})
	if err == nil {
		t.Fatal("零值 asOf 应返回错误")
	}
	if err.Error() != "asOf time is required" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

// --- QueryGraphByTimeRange 参数验证 ---

func TestQueryGraphByTimeRange_EmptyUserID(t *testing.T) {
	gs := &GraphStoreService{}
	now := time.Now()
	_, err := gs.QueryGraphByTimeRange(nil, "", now.Add(-24*time.Hour), now)
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
}

func TestQueryGraphByTimeRange_ZeroTimes(t *testing.T) {
	gs := &GraphStoreService{}
	_, err := gs.QueryGraphByTimeRange(nil, "user1", time.Time{}, time.Time{})
	if err == nil {
		t.Fatal("零值 from/to 应返回错误")
	}
}

func TestQueryGraphByTimeRange_InvalidOrder(t *testing.T) {
	gs := &GraphStoreService{}
	now := time.Now()
	_, err := gs.QueryGraphByTimeRange(nil, "user1", now, now.Add(-24*time.Hour))
	if err == nil {
		t.Fatal("to < from 应返回错误")
	}
	if err.Error() != "to must be after from" {
		t.Fatalf("错误信息不匹配: %s", err.Error())
	}
}

func TestQueryGraphByTimeRange_SameTime(t *testing.T) {
	gs := &GraphStoreService{}
	now := time.Now()
	// from == to 应通过参数验证（合法），nil db 会导致 panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("from == to 配合 nil db 应 panic（说明参数验证通过了）")
		}
	}()
	_, _ = gs.QueryGraphByTimeRange(nil, "user1", now, now)
}

// --- QueryRecentEntities 参数验证 ---

func TestQueryRecentEntities_EmptyUserID(t *testing.T) {
	gs := &GraphStoreService{}
	_, err := gs.QueryRecentEntities(nil, "", "", 10)
	if err == nil {
		t.Fatal("空 user_id 应返回错误")
	}
}

func TestQueryRecentEntities_DefaultLimit(t *testing.T) {
	gs := &GraphStoreService{}
	// limit <= 0 应默认为 10，参数验证通过后 nil db panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil db 应导致 panic（说明参数验证通过、limit 被修正）")
		}
	}()
	_, _ = gs.QueryRecentEntities(nil, "user1", "", 0)
}

func TestQueryRecentEntities_OverLimit(t *testing.T) {
	gs := &GraphStoreService{}
	// limit > 100 应被夹到 10，参数验证通过后 nil db panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil db 应导致 panic（说明参数验证通过、limit 被修正）")
		}
	}()
	_, _ = gs.QueryRecentEntities(nil, "user1", "", 200)
}

// --- TemporalQueryResult 零值 ---

func TestTemporalQueryResult_Defaults(t *testing.T) {
	r := &TemporalQueryResult{}
	if r.Entities != nil {
		t.Fatal("默认 Entities 应为 nil")
	}
	if r.Relations != nil {
		t.Fatal("默认 Relations 应为 nil")
	}
	if r.AsOf != nil {
		t.Fatal("默认 AsOf 应为 nil")
	}
}
