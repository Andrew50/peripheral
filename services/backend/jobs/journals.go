package jobs

import (
	"backend/utils"
	"context"
	"fmt"
	"time"
)

func pushJournals(conn *utils.Conn, year int, month time.Month, day int) {
	currentDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	userIDs, err := getAllUserIDs(conn)
	if err != nil {
		fmt.Printf("Error fetching user ids: %v\n", err)
		return
	}
	for _, userID := range userIDs {
		_, err := conn.DB.Exec(context.Background(), ` INSERT INTO journals (timestamp, userId, entry)
            VALUES ($1, $2, $3);
        `, currentDate, userID, "{}") // Assuming an empty JSON object for 'entry'
		if err != nil {
			fmt.Printf(" 0b1q Error inserting journal for userId %d: %v\n", userID, err)
		} else {
			fmt.Printf("1niuv %d on %v\n", userID, currentDate)
		}
	}
}

func getAllUserIDs(conn *utils.Conn) ([]int, error) {
	rows, err := conn.DB.Query(context.Background(), "SELECT userId FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var userIDs []int
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return userIDs, nil
}
