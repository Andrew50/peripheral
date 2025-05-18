package marketdata

import (
	"backend/internal/data"
	"backend/internal/data/polygon"
	"context"
	"fmt"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

func updateOHLCVGeneric(conn *data.Conn, table, timespan string) error {
	ctx := context.Background()
	var maxTs time.Time
	row := conn.DB.QueryRow(ctx, fmt.Sprintf("SELECT MAX(timestamp) FROM %s", table))
	_ = row.Scan(&maxTs)
	if maxTs.IsZero() {
		maxTs = time.Date(2003, 10, 1, 0, 0, 0, 0, time.UTC)
	}
	rows, err := conn.DB.Query(ctx, "SELECT securityid, ticker FROM securities")
	if err != nil {
		return err
	}
	defer rows.Close()
	end := time.Now().UTC()
	for rows.Next() {
		var id int
		var ticker string
		if err := rows.Scan(&id, &ticker); err != nil {
			continue
		}
		iter, err := polygon.GetAggsData(conn.Polygon, ticker, 1, timespan, models.Millis(maxTs), models.Millis(end), 1000, "asc", true)
		if err != nil {
			continue
		}
		for iter.Next() {
			agg := iter.Item()
			ts := time.Time(agg.Timestamp)
			_, _ = conn.DB.Exec(ctx,
				fmt.Sprintf(`INSERT INTO %s (timestamp, securityid, open, high, low, close, volume)
                VALUES ($1,$2,$3,$4,$5,$6,$7)
                ON CONFLICT (securityid, timestamp) DO NOTHING`, table),
				ts, id, agg.Open, agg.High, agg.Low, agg.Close, agg.Volume)
		}
		if err := iter.Err(); err != nil {
			continue
		}
	}


	return nil
}

func UpdateSecondOHLCV(conn *data.Conn) error { return updateOHLCVGeneric(conn, "ohlcv_1s", "second") }
func UpdateMinuteOHLCV(conn *data.Conn) error { return updateOHLCVGeneric(conn, "ohlcv_1", "minute") }
func UpdateHourlyOHLCV(conn *data.Conn) error { return updateOHLCVGeneric(conn, "ohlcv_1h", "hour") }
func UpdateWeeklyOHLCV(conn *data.Conn) error { return updateOHLCVGeneric(conn, "ohlcv_1w", "week") }
