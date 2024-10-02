package utils

import (
    "fmt"
    "context"
)

func CheckSampleQueue(conn *Conn, setupId int, addedSample bool) {
    if addedSample {
        // Update untrainedSampleChanges and sampleSize if a new sample is added
        _, err := conn.DB.Exec(context.Background(), `
            UPDATE setups 
            SET untrainedSampleChanges = untrainedSampleChanges + 1, 
                sampleSize = sampleSize + 1
            WHERE setupId = $1`, setupId)
        if err != nil {
            fmt.Printf("Error updating sample counts: %v\n", err)
            return
        }
    }
    checkModel(conn, setupId)

    var queueLength int
    err := conn.DB.QueryRow(context.Background(), `
        SELECT COUNT(*) 
        FROM samples 
        WHERE setupId = $1 AND label IS NULL`, setupId).Scan(&queueLength)
    if err != nil {
        fmt.Printf("Error checking queue length: %v\n", err)
        return
    }
    if queueLength < 20 {
        queueRunningKey := fmt.Sprintf("%d_queue_running", setupId)
        queueRunning := conn.Cache.Get(context.Background(), queueRunningKey).Val()
        if queueRunning != "true" {
            conn.Cache.Set(context.Background(), queueRunningKey, "true", 0)
            _, err := Queue(conn, "refillTrainerQueue", map[string]interface{}{
                "setupId": setupId,
            })

            if err != nil {
                fmt.Printf("Error enqueuing refillQueue: %v\n", err)
                conn.Cache.Del(context.Background(), queueRunningKey)
                return
            }
            fmt.Printf("Enqueued refillQueue for setupId: %d\n", setupId)
        }
    }
}

func checkModel(conn *Conn, setupId int) {
    var untrainedSampleChanges int
    var sampleSize int

    // Retrieve untrainedSampleChanges and sampleSize from the database
    err := conn.DB.QueryRow(context.Background(), `
        SELECT untrainedSampleChanges, sampleSize
        FROM setups
        WHERE setupId = $1`, setupId).Scan(&untrainedSampleChanges, &sampleSize)
    if err != nil {
        fmt.Printf("Error retrieving model info: %v\n", err)
        return
    }

    // Check if untrained samples exceed 20 or a certain percentage of sampleSize
    if untrainedSampleChanges > 1 || float64(untrainedSampleChanges)/float64(sampleSize) > 0.05 {
        trainRunningKey := fmt.Sprintf("%d_train_running", setupId)
        trainRunning := conn.Cache.Get(context.Background(), trainRunningKey).Val()

        // Add "train" to the queue if not already running
        if trainRunning != "true" {
            conn.Cache.Set(context.Background(), trainRunningKey, "true", 0)
            _, err := Queue(conn, "train", map[string]interface{}{
                "setupId": setupId,
            })
            if err != nil {
                fmt.Printf("Error enqueuing train task: %v\n", err)
                conn.Cache.Del(context.Background(), trainRunningKey)
                return
            }
            fmt.Printf("Enqueued train task for setupId: %d\n", setupId)
        }
    }
}

