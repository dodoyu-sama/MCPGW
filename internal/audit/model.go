package audit

import "time"

type CallRecord struct {
	ID        int64     `db:"id" json:"id"`
	ClientID  string    `db:"client_id" json:"client_id"`
	ToolName  string    `db:"tool_name" json:"tool_name"`
	Params    string    `db:"params" json:"params"`         // 脱敏后的参数 JSON
	RawParams string    `db:"raw_params" json:"raw_params"` // 原始（未脱敏）请求参数，用于回放
	Result    string    `db:"result" json:"result"`         // 脱敏后的结果 JSON
	RawResult string    `db:"raw_result" json:"raw_result"` // 原始（未脱敏）上游返回，用于回放/核对
	ErrorMsg  string    `db:"error_msg" json:"error_msg"`
	LatencyMs int64     `db:"latency_ms" json:"latency_ms"`
	Timestamp time.Time `db:"timestamp" json:"timestamp"`
}

type QueryOpts struct {
	ClientID string
	ToolName string
	Since    time.Time
	Until    time.Time
	Limit    int
}

type StatsOpts struct {
	Since time.Time
	Until time.Time
}

type Stats struct {
	TotalCalls   int64            `json:"total_calls"`
	ErrorCount   int64            `json:"error_count"`
	AvgLatencyMs float64          `json:"avg_latency_ms"`
	ToolCounts   map[string]int64 `json:"tool_counts"`
}

// Store is the persistence interface for audit records.
type Store interface {
	Insert(r *CallRecord) error
	Query(opts QueryOpts) ([]CallRecord, error)
	Get(id int64) (*CallRecord, error)
	Stats(opts StatsOpts) (*Stats, error)
	Close() error
}
