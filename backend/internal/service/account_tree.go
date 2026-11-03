package service

import (
	"context"
)

// AccountTreeSummaryItem 是账号树视图中一个分组节点的统计。GroupID 为 0 表示"未分组"。
// 同一账号可属于多个分组，会在每个分组下各计一次。
type AccountTreeSummaryItem struct {
	GroupID     int64 `json:"group_id"`
	Total       int64 `json:"total"`
	Active      int64 `json:"active"`
	Schedulable int64 `json:"schedulable"`
	Error       int64 `json:"error"`
}

// AccountTreeSummaryFilter 树视图统计的过滤条件（与账号列表的基础过滤一致）。
type AccountTreeSummaryFilter struct {
	Platform    string
	AccountType string
	Search      string
}

// AccountTreeRepository 账号树视图统计查询。
type AccountTreeRepository interface {
	GroupSummary(ctx context.Context, filter AccountTreeSummaryFilter) ([]AccountTreeSummaryItem, error)
}

// AccountTreeService 为账号列表的"分组 → 账号"树形视图提供节点统计。
type AccountTreeService struct {
	repo AccountTreeRepository
}

func NewAccountTreeService(repo AccountTreeRepository) *AccountTreeService {
	return &AccountTreeService{repo: repo}
}

func (s *AccountTreeService) GroupSummary(ctx context.Context, filter AccountTreeSummaryFilter) ([]AccountTreeSummaryItem, error) {
	return s.repo.GroupSummary(ctx, filter)
}
