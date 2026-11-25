package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"llm-api-uptime/internal/domain"
)

type HeartbeatRepo struct {
	writer *sql.DB
	reader *sql.DB
}

func NewHeartbeatRepo(writer, reader *sql.DB) *HeartbeatRepo {
	return &HeartbeatRepo{writer: writer, reader: reader}
}

func (r *HeartbeatRepo) Save(ctx context.Context, hb *domain.Heartbeat) error {
	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var modelsOK sql.NullInt64
	if hb.ModelsOK != nil {
		if *hb.ModelsOK {
			modelsOK = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			modelsOK = sql.NullInt64{Int64: 0, Valid: true}
		}
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO heartbeats (
			monitor_id, status, stage_failed, http_status, models_ok, models_latency_ms,
			completion_ok, used_stream, ttft_ms, total_latency_ms, output_tokens,
			tokens_per_sec, tps_method, error_message, checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		hb.MonitorID, string(hb.Status), hb.StageFailed, hb.HTTPStatus, modelsOK, hb.ModelsLatencyMs,
		boolToInt(hb.CompletionOK), boolToInt(hb.UsedStream), hb.TTFTMs, hb.TotalLatencyMs, hb.OutputTokens,
		hb.TokensPerSec, hb.TPSMethod, hb.ErrorMessage, hb.CheckedAt,
	)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil {
		hb.ID = id
	}

	checkedTime, err := time.Parse(time.RFC3339Nano, hb.CheckedAt)
	if err != nil {
		checkedTime = time.Now().UTC()
	}
	bucketHour := checkedTime.UTC().Truncate(time.Hour).Format(time.RFC3339)

	upInc, degInc, downInc := 0, 0, 0
	switch hb.Status {
	case domain.StatusUp:
		upInc = 1
	case domain.StatusDegraded:
		degInc = 1
	default:
		downInc = 1
	}

	var ttftSum float64
	var ttftCnt int
	if hb.TTFTMs != nil {
		ttftSum = float64(*hb.TTFTMs)
		ttftCnt = 1
	}
	var totalSum float64
	var totalCnt int
	if hb.TotalLatencyMs != nil {
		totalSum = float64(*hb.TotalLatencyMs)
		totalCnt = 1
	}
	var tpsSum float64
	var tpsCnt int
	if hb.TokensPerSec != nil {
		tpsSum = *hb.TokensPerSec
		tpsCnt = 1
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO hourly_stats (
			monitor_id, bucket_hour, total_checks, up_checks, degraded_checks, down_checks,
			sum_ttft_ms, ttft_count, sum_total_ms, total_ms_count, sum_tps, tps_count
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(monitor_id, bucket_hour) DO UPDATE SET
			total_checks = total_checks + 1,
			up_checks = up_checks + excluded.up_checks,
			degraded_checks = degraded_checks + excluded.degraded_checks,
			down_checks = down_checks + excluded.down_checks,
			sum_ttft_ms = sum_ttft_ms + excluded.sum_ttft_ms,
			ttft_count = ttft_count + excluded.ttft_count,
			sum_total_ms = sum_total_ms + excluded.sum_total_ms,
			total_ms_count = total_ms_count + excluded.total_ms_count,
			sum_tps = sum_tps + excluded.sum_tps,
			tps_count = tps_count + excluded.tps_count
	`, hb.MonitorID, bucketHour, upInc, degInc, downInc, ttftSum, ttftCnt, totalSum, totalCnt, tpsSum, tpsCnt)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *HeartbeatRepo) ListRecent(ctx context.Context, monitorID string, limit int) ([]domain.Heartbeat, error) {
	if limit <= 0 || limit > 500 {
		limit = 60
	}
	rows, err := r.reader.QueryContext(ctx, `
		SELECT id, monitor_id, status, stage_failed, http_status, models_ok, models_latency_ms,
			completion_ok, used_stream, ttft_ms, total_latency_ms, output_tokens,
			tokens_per_sec, tps_method, error_message, checked_at
		FROM heartbeats
		WHERE monitor_id = ?
		ORDER BY checked_at DESC
		LIMIT ?
	`, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	desc := make([]domain.Heartbeat, 0)
	for rows.Next() {
		hb, err := scanHeartbeat(rows)
		if err != nil {
			return nil, err
		}
		desc = append(desc, hb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to chronological order (oldest -> newest) for charts & bars.
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

func (r *HeartbeatRepo) ListSince(ctx context.Context, monitorID string, since time.Time, limit int) ([]domain.Heartbeat, error) {
	if limit <= 0 || limit > 2000 {
		limit = 1000
	}
	rows, err := r.reader.QueryContext(ctx, `
		SELECT id, monitor_id, status, stage_failed, http_status, models_ok, models_latency_ms,
			completion_ok, used_stream, ttft_ms, total_latency_ms, output_tokens,
			tokens_per_sec, tps_method, error_message, checked_at
		FROM heartbeats
		WHERE monitor_id = ? AND checked_at >= ?
		ORDER BY checked_at ASC
		LIMIT ?
	`, monitorID, since.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Heartbeat, 0)
	for rows.Next() {
		hb, err := scanHeartbeat(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, hb)
	}
	return items, rows.Err()
}

func (r *HeartbeatRepo) ListHourlySince(ctx context.Context, monitorID string, since time.Time) ([]domain.HourlyStat, error) {
	rows, err := r.reader.QueryContext(ctx, `
		SELECT monitor_id, bucket_hour, total_checks, up_checks, degraded_checks, down_checks,
			sum_ttft_ms, ttft_count, sum_total_ms, total_ms_count, sum_tps, tps_count
		FROM hourly_stats
		WHERE monitor_id = ? AND bucket_hour >= ?
		ORDER BY bucket_hour ASC
	`, monitorID, since.UTC().Truncate(time.Hour).Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]domain.HourlyStat, 0)
	for rows.Next() {
		var st domain.HourlyStat
		var sumTTFT, sumTotal, sumTPS float64
		var ttftCnt, totalCnt, tpsCnt int
		if err := rows.Scan(
			&st.MonitorID, &st.BucketHour, &st.TotalChecks, &st.UpChecks, &st.DegradedChecks, &st.DownChecks,
			&sumTTFT, &ttftCnt, &sumTotal, &totalCnt, &sumTPS, &tpsCnt,
		); err != nil {
			return nil, err
		}
		if ttftCnt > 0 {
			v := sumTTFT / float64(ttftCnt)
			st.AvgTTFTMs = &v
		}
		if totalCnt > 0 {
			v := sumTotal / float64(totalCnt)
			st.AvgTotalMs = &v
		}
		if tpsCnt > 0 {
			v := sumTPS / float64(tpsCnt)
			st.AvgTPS = &v
		}
		if st.TotalChecks > 0 {
			st.UptimePct = float64(st.UpChecks+st.DegradedChecks) * 100.0 / float64(st.TotalChecks)
		}
		stats = append(stats, st)
	}
	return stats, rows.Err()
}

type WindowSummary struct {
	UptimePct  float64
	AvgTPS     *float64
	AvgTTFTMs  *float64
	TotalCount int
}

func (r *HeartbeatRepo) SummarizeWindow(ctx context.Context, monitorID string, since time.Time) (WindowSummary, error) {
	var total, up, degraded int
	var sumTTFT, sumTPS sql.NullFloat64
	var ttftCnt, tpsCnt int

	err := r.reader.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(total_checks), 0),
			COALESCE(SUM(up_checks), 0),
			COALESCE(SUM(degraded_checks), 0),
			SUM(sum_ttft_ms),
			COALESCE(SUM(ttft_count), 0),
			SUM(sum_tps),
			COALESCE(SUM(tps_count), 0)
		FROM hourly_stats
		WHERE monitor_id = ? AND bucket_hour >= ?
	`, monitorID, since.UTC().Truncate(time.Hour).Format(time.RFC3339)).Scan(
		&total, &up, &degraded, &sumTTFT, &ttftCnt, &sumTPS, &tpsCnt,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return WindowSummary{}, err
	}

	out := WindowSummary{TotalCount: total, UptimePct: 100.0}
	if total > 0 {
		out.UptimePct = float64(up+degraded) * 100.0 / float64(total)
	}
	if ttftCnt > 0 && sumTTFT.Valid {
		v := sumTTFT.Float64 / float64(ttftCnt)
		out.AvgTTFTMs = &v
	}
	if tpsCnt > 0 && sumTPS.Valid {
		v := sumTPS.Float64 / float64(tpsCnt)
		out.AvgTPS = &v
	}
	return out, nil
}

func (r *HeartbeatRepo) PruneOlderThan(ctx context.Context, rawCutoff, hourlyCutoff time.Time) (int64, error) {
	res, err := r.writer.ExecContext(ctx, `DELETE FROM heartbeats WHERE checked_at < ?`, rawCutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	deleted, _ := res.RowsAffected()

	_, err = r.writer.ExecContext(ctx, `DELETE FROM hourly_stats WHERE bucket_hour < ?`, hourlyCutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return deleted, err
	}
	return deleted, nil
}

func scanHeartbeat(s scanner) (domain.Heartbeat, error) {
	var hb domain.Heartbeat
	var statusStr string
	var modelsOK sql.NullInt64
	var modelsLat sql.NullInt64
	var compOK, usedStream int
	var ttft, totalLat sql.NullInt64
	var outTokens sql.NullInt64
	var tps sql.NullFloat64

	err := s.Scan(
		&hb.ID, &hb.MonitorID, &statusStr, &hb.StageFailed, &hb.HTTPStatus,
		&modelsOK, &modelsLat, &compOK, &usedStream, &ttft, &totalLat,
		&outTokens, &tps, &hb.TPSMethod, &hb.ErrorMessage, &hb.CheckedAt,
	)
	if err != nil {
		return domain.Heartbeat{}, err
	}

	hb.Status = domain.Status(statusStr)
	hb.CompletionOK = compOK == 1
	hb.UsedStream = usedStream == 1
	if modelsOK.Valid {
		v := modelsOK.Int64 == 1
		hb.ModelsOK = &v
	}
	if modelsLat.Valid {
		v := modelsLat.Int64
		hb.ModelsLatencyMs = &v
	}
	if ttft.Valid {
		v := ttft.Int64
		hb.TTFTMs = &v
	}
	if totalLat.Valid {
		v := totalLat.Int64
		hb.TotalLatencyMs = &v
	}
	if outTokens.Valid {
		v := int(outTokens.Int64)
		hb.OutputTokens = &v
	}
	if tps.Valid {
		v := tps.Float64
		hb.TokensPerSec = &v
	}
	return hb, nil
}
