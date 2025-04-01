package tools

/*
sqlQuery, err := jsonQueryToSQL(llmResponse)
    if err != nil {
        return nil, fmt.Errorf("error converting JSON to SQL: %w", err)
    }

    // Execute the SQL query and get the results
    results, err := executeQuery(conn, sqlQuery)
    if err != nil {
        return nil, fmt.Errorf("error executing SQL query: %w", err)
    }

    return map[string]interface{}{
        "parsed_query": llmResponse,
        "sql_query": sqlQuery,
        "results": results,
    }, nil
// jsonQueryToSQL converts LLM JSON output to SQL query
func jsonQueryToSQL(jsonStr string) (string, error) {
    var pq ParsedQuery
    if err := json.Unmarshal([]byte(jsonStr), &pq); err != nil {
        return "", fmt.Errorf("error parsing JSON: %w", err)
    }

    // Base query components
    selectClause := "SELECT timestamp, ticker, open, high, low, close, volume"
    fromClause := "FROM daily_ohlcv"
    whereClauses := []string{}

    // Handle stock selection
    stocks := pq.Stocks
    if stocks.Universe == "list" && len(stocks.Include) > 0 {
        tickers := fmt.Sprintf("'%s'", strings.Join(stocks.Include, "','"))
        whereClauses = append(whereClauses, fmt.Sprintf("ticker IN (%s)", tickers))
    }
    for key, filter := range stocks.Filters {
        if key == "volume" { // Add more filter types as needed
            whereClauses = append(whereClauses, fmt.Sprintf("volume %s %f", filter.Operator, filter.Value))
        }
    }

    // Handle conditions
    for _, cond := range pq.Conditions {
        conditionSQL := conditionToSQL(cond)
        whereClauses = append(whereClauses, conditionSQL)
    }

    // Handle sequence
    if pq.Sequence.Condition == "dropped 5%" && pq.Sequence.Window > 0 {
        window := pq.Sequence.Window
        seqSQL := fmt.Sprintf("(LEAD(close, %d) OVER (PARTITION BY ticker ORDER BY timestamp) / close - 1) < -0.05", window)
        whereClauses = append(whereClauses, seqSQL)
    }

    // Handle date range
    dateRange := pq.DateRange
    if dateRange.Start != "" && dateRange.End != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("timestamp BETWEEN '%s' AND '%s'", dateRange.Start, dateRange.End))
    } else {
        whereClauses = append(whereClauses, "timestamp >= NOW() - INTERVAL '1 year'")
    }

    // Assemble the full query
    whereClause := ""
    if len(whereClauses) > 0 {
        whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
    }
    sqlQuery := fmt.Sprintf("%s %s %s ORDER BY timestamp DESC", selectClause, fromClause, whereClause)
    return sqlQuery, nil
} */
// Helper functions

/*
func conditionToSQL(cond Condition) string {
    lhsSQL := fieldWithOffsetSQL(cond.LHS)
    var rhsSQL string

    if cond.RHS.Field != "" {
        rhsSQL = fieldWithOffsetSQL(FieldWithOffset{Field: cond.RHS.Field, Offset: cond.RHS.Offset})
    } else if cond.RHS.Indicator != "" {
        rhsSQL = indicatorWithOffsetSQL(cond.RHS.Indicator, cond.RHS.Period, cond.RHS.Offset)
    } else {
        rhsSQL = fmt.Sprintf("%v", cond.RHS.Value)
    }

    return fmt.Sprintf("%s %s %s", lhsSQL, cond.Operation, rhsSQL)
}

func fieldWithOffsetSQL(f FieldWithOffset) string {
    if f.Offset == 0 {
        return f.Field
    }
    lag := -f.Offset // Negative offset means past data
    return fmt.Sprintf("LAG(%s, %d) OVER (PARTITION BY ticker ORDER BY timestamp)", f.Field, lag)
}

func indicatorWithOffsetSQL(ind string, period int, offset int) string {
    if ind == "SMA" {
        smaSQL := fmt.Sprintf("AVG(close) OVER (PARTITION BY ticker ORDER BY timestamp ROWS BETWEEN %d PRECEDING AND CURRENT ROW)", period-1)
        if offset == 0 {
            return smaSQL
        }
        lag := -offset
        return fmt.Sprintf("LAG(%s, %d) OVER (PARTITION BY ticker ORDER BY timestamp)", smaSQL, lag)
    }
    return "" // Add support for other indicators here
}

// executeQuery executes the SQL query and returns the results
func executeQuery(conn *utils.Conn, sqlQuery string) ([]map[string]interface{}, error) {
    ctx := context.Background()

    // Execute the query
    rows, err := conn.DB.Query(ctx, sqlQuery)
    if err != nil {
        return nil, fmt.Errorf("error executing SQL query: %w", err)
    }
    defer rows.Close()

    // Get column names
    fieldDescriptions := rows.FieldDescriptions()
    columnNames := make([]string, len(fieldDescriptions))
    for i, fd := range fieldDescriptions {
        columnNames[i] = string(fd.Name)
    }

    // Store results
    results := []map[string]interface{}{}

    // Iterate through the rows
    for rows.Next() {
        // Create a slice of interface{} to hold the values
        values := make([]interface{}, len(columnNames))
        valuePtrs := make([]interface{}, len(columnNames))

        // Create a pointer to each value
        for i := range values {
            valuePtrs[i] = &values[i]
        }

        // Scan the row into the interface{} slice
        if err := rows.Scan(valuePtrs...); err != nil {
            return nil, fmt.Errorf("error scanning row: %w", err)
        }

        // Create a map for this row's data
        rowData := make(map[string]interface{})

        // Store each column value in the map
        for i, col := range columnNames {
            val := values[i]

            // Convert time.Time values to strings for JSON compatibility
            if timeVal, ok := val.(time.Time); ok {
                rowData[col] = timeVal.Format(time.RFC3339)
            } else {
                rowData[col] = val
            }
        }

        results = append(results, rowData)
    }

    // Check for errors from iterating over rows
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating over rows: %w", err)
    }

    return results, nil
}




*/
