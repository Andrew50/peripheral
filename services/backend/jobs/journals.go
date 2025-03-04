package jobs

import (
    "time"
    "backend/utils"
    "fmt"
    "context"
)

func pushJournals(conn *utils.Conn, year int, month time.Month, day int) {
    currentDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
    userIds, err := getAllUserIds(conn)
    if err != nil {
        fmt.Printf("Error fetching user ids: %v\n", err)
        return
    }
    for _, userId := range userIds {
        _, err := conn.DB.Exec(context.Background(),` INSERT INTO journals (timestamp, userId, entry)
            VALUES ($1, $2, $3);
        `, currentDate, userId, "{}") // Assuming an empty JSON object for 'entry'
        if err != nil {
            fmt.Printf(" 0b1q Error inserting journal for userId %d: %v\n", userId, err)
        } else {
            fmt.Printf("1niuv %d on %v\n", userId, currentDate)
        }
    }
}

func getAllUserIds(conn *utils.Conn) ([]int, error) {
    rows, err := conn.DB.Query(context.Background(),"SELECT userId FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var userIds []int
    for rows.Next() {
        var userId int
        if err := rows.Scan(&userId); err != nil {
            return nil, err
        }
        userIds = append(userIds, userId)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return userIds, nil
}


