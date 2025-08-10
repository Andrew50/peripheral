package marketdata

import (
	"backend/internal/data"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

const (
	shortDataMinISO    = "2003-01-01"
	redisInterestKey   = "short_data:last_interest_date"
	redisVolumeKey     = "short_data:last_volume_date"
	shortDataBatchSize = 200
)

// UpdateShortData ingests Polygon short interest first, then short volume, resumably into short_data.
func UpdateShortData(conn *data.Conn) error {
	ctx := context.Background()

	log.Printf("🚀 ShortData: starting interest pass")
	if err := updateShortInterest(ctx, conn); err != nil {
		return err
	}

	log.Printf("🚀 ShortData: starting volume pass")
	if err := updateShortVolume(ctx, conn); err != nil {
		return err
	}

	log.Printf("✅ ShortData: both passes completed successfully")
	return nil
}

// shortRow represents one record for upsert into short_data.
type shortRow struct {
	Ticker   string
	DataDate *time.Time

	ShortInterest  *int64
	AvgDailyVolume *int64
	DaysToCover    *float64

	ShortVolume      *int64
	ShortVolumeRatio *float64
	TotalVolume      *int64
	NonExemptVolume  *int64
	ExemptVolume     *int64
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getDBMaxInterestDate(ctx context.Context, conn *data.Conn) (string, error) {
	const q = `SELECT COALESCE(TO_CHAR(MAX(data_date), 'YYYY-MM-DD'), '') FROM short_data WHERE short_interest IS NOT NULL`
	var iso string
	if err := conn.DB.QueryRow(ctx, q).Scan(&iso); err != nil {
		return "", err
	}
	return iso, nil
}

func getDBMaxVolumeDate(ctx context.Context, conn *data.Conn) (string, error) {
	const q = `SELECT COALESCE(TO_CHAR(MAX(data_date), 'YYYY-MM-DD'), '') FROM short_data WHERE short_volume IS NOT NULL`
	var iso string
	if err := conn.DB.QueryRow(ctx, q).Scan(&iso); err != nil {
		return "", err
	}
	return iso, nil
}

func updateShortInterest(ctx context.Context, conn *data.Conn) error {
	startISO := shortDataMinISO
	if dbMax, err := getDBMaxInterestDate(ctx, conn); err == nil && dbMax != "" {
		startISO = dbMax
	}
	// Optional Redis hint
	if hint, err := conn.Cache.Get(ctx, redisInterestKey).Result(); err == nil && hint != "" {
		log.Printf("ℹ️ ShortData interest redis hint: %s (DB start: %s)", hint, startISO)
	} else {
		log.Printf("ℹ️ ShortData interest DB start: %s", startISO)
	}

	params := models.ListShortInterestParams{}.
		WithOrder(models.Asc).
		WithLimit(1000)
	// settlement_date >= startISO
	params = params.WithSettlementDate(models.GTE, startISO)

	// Use REST client (short endpoints are on standard client)
	iter := conn.Polygon.ListShortInterest(ctx, params)

	var (
		rows       []*shortRow
		lastISO    string
		batchCount int
		startTime  = time.Now()
	)
	log.Printf("📈 ShortData interest: starting from settlement_date >= %s", startISO)

	for iter.Next() {
		it := iter.Item()

		// Extract fields from iterator item
		ticker := safeStr(it.Ticker)
		if ticker == "" {
			continue
		}
		if d := safeStr(it.SettlementDate); d != "" {
			lastISO = d
		}

		r := &shortRow{Ticker: ticker, DataDate: parseISODate(safeStr(it.SettlementDate))}

		// Assign numeric pointers directly; nil means NULL
		r.ShortInterest = it.ShortInterest
		// Prefer AvgDailyVolume if available in SDK; keep as nil-safe
		r.AvgDailyVolume = it.AvgDailyVolume
		r.DaysToCover = it.DaysToCover

		rows = append(rows, r)
		if len(rows) >= shortDataBatchSize {
			if err := upsertShortRows(ctx, conn, rows); err != nil {
				return fmt.Errorf("short interest upsert failed: %w", err)
			}
			if lastISO != "" {
				_ = conn.Cache.Set(ctx, redisInterestKey, lastISO, 0).Err()
			}
			rows = rows[:0]
			batchCount++
			logProgressEstimate("ShortData interest", batchCount, 20, startISO, lastISO, startTime)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("polygon short interest iterator error: %w", err)
	}

	if len(rows) > 0 {
		if err := upsertShortRows(ctx, conn, rows); err != nil {
			return fmt.Errorf("short interest final upsert failed: %w", err)
		}
		if lastISO != "" {
			_ = conn.Cache.Set(ctx, redisInterestKey, lastISO, 0).Err()
		}
		batchCount++
		logProgressEstimate("ShortData interest", batchCount, 20, startISO, lastISO, startTime)
	}

	if lastISO == "" {
		lastISO = startISO
	}
	log.Printf("✅ ShortData interest: complete through settlement_date %s", lastISO)
	return nil
}

func updateShortVolume(ctx context.Context, conn *data.Conn) error {
	startISO := shortDataMinISO
	if dbMax, err := getDBMaxVolumeDate(ctx, conn); err == nil && dbMax != "" {
		startISO = dbMax
	}
	// Optional Redis hint
	if hint, err := conn.Cache.Get(ctx, redisVolumeKey).Result(); err == nil && hint != "" {
		log.Printf("ℹ️ ShortData volume redis hint: %s (DB start: %s)", hint, startISO)
	} else {
		log.Printf("ℹ️ ShortData volume DB start: %s", startISO)
	}

	params := models.ListShortVolumeParams{}.
		WithOrder(models.Asc).
		WithLimit(1000)
	// date >= startISO
	params = params.WithDate(models.GTE, startISO)

	iter := conn.Polygon.ListShortVolume(ctx, params)

	var (
		rows       []*shortRow
		lastISO    string
		batchCount int
		startTime  = time.Now()
	)
	log.Printf("📊 ShortData volume: starting from date >= %s", startISO)

	for iter.Next() {
		it := iter.Item()
		ticker := safeStr(it.Ticker)
		if ticker == "" {
			continue
		}
		if d := safeStr(it.Date); d != "" {
			lastISO = d
		}

		r := &shortRow{Ticker: ticker, DataDate: parseISODate(safeStr(it.Date))}

		r.ShortVolume = it.ShortVolume
		r.ShortVolumeRatio = it.ShortVolumeRatio
		r.TotalVolume = it.TotalVolume
		r.NonExemptVolume = it.NonExemptVolume
		r.ExemptVolume = it.ExemptVolume

		rows = append(rows, r)
		if len(rows) >= shortDataBatchSize {
			if err := upsertShortRows(ctx, conn, rows); err != nil {
				return fmt.Errorf("short volume upsert failed: %w", err)
			}
			if lastISO != "" {
				_ = conn.Cache.Set(ctx, redisVolumeKey, lastISO, 0).Err()
			}
			rows = rows[:0]
			batchCount++
			logProgressEstimate("ShortData volume", batchCount, 20, startISO, lastISO, startTime)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("polygon short volume iterator error: %w", err)
	}

	if len(rows) > 0 {
		if err := upsertShortRows(ctx, conn, rows); err != nil {
			return fmt.Errorf("short volume final upsert failed: %w", err)
		}
		if lastISO != "" {
			_ = conn.Cache.Set(ctx, redisVolumeKey, lastISO, 0).Err()
		}
		batchCount++
		logProgressEstimate("ShortData volume", batchCount, 20, startISO, lastISO, startTime)
	}

	if lastISO == "" {
		lastISO = startISO
	}
	log.Printf("✅ ShortData volume: complete through date %s", lastISO)
	return nil
}

func upsertShortRows(ctx context.Context, conn *data.Conn, rows []*shortRow) error {
	if len(rows) == 0 {
		return nil
	}

	var (
		sb   strings.Builder
		args []interface{}
		idx  = 1
	)

	sb.WriteString("INSERT INTO short_data (" +
		"ticker,data_date," +
		"short_interest,avg_daily_volume,days_to_cover," +
		"short_volume,short_volume_ratio,total_volume,non_exempt_volume,exempt_volume" +
		") VALUES ")

	add := func(v interface{}) {
		args = append(args, v)
		idx++
	}

	for i, r := range rows {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(")
		// ticker, date
		sb.WriteString(fmt.Sprintf("$%d,$%d,", idx, idx+1))
		add(r.Ticker)
		add(r.DataDate)

		// interest 3
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,", idx, idx+1, idx+2))
		add(r.ShortInterest)
		add(r.AvgDailyVolume)
		add(r.DaysToCover)

		// volume 5
		sb.WriteString(fmt.Sprintf("$%d,$%d,$%d,$%d,$%d)", idx, idx+1, idx+2, idx+3, idx+4))
		add(r.ShortVolume)
		add(r.ShortVolumeRatio)
		add(r.TotalVolume)
		add(r.NonExemptVolume)
		add(r.ExemptVolume)
	}

	// COALESCE to avoid null-wiping previously ingested fields, always bump ingested_at
	sb.WriteString(" ON CONFLICT (ticker, data_date) DO UPDATE SET ")
	setCols := []string{
		"short_interest=COALESCE(EXCLUDED.short_interest, short_data.short_interest)",
		"avg_daily_volume=COALESCE(EXCLUDED.avg_daily_volume, short_data.avg_daily_volume)",
		"days_to_cover=COALESCE(EXCLUDED.days_to_cover, short_data.days_to_cover)",
		"short_volume=COALESCE(EXCLUDED.short_volume, short_data.short_volume)",
		"short_volume_ratio=COALESCE(EXCLUDED.short_volume_ratio, short_data.short_volume_ratio)",
		"total_volume=COALESCE(EXCLUDED.total_volume, short_data.total_volume)",
		"non_exempt_volume=COALESCE(EXCLUDED.non_exempt_volume, short_data.non_exempt_volume)",
		"exempt_volume=COALESCE(EXCLUDED.exempt_volume, short_data.exempt_volume)",
		"ingested_at=now()",
	}
	sb.WriteString(strings.Join(setCols, ","))

	sql := sb.String()
	if _, err := data.ExecWithRetry(ctx, conn.DB, sql, args...); err != nil {
		return err
	}
	//log.Printf("📥 ShortData: upserted %d rows (batch)", len(rows))
	return nil
}
