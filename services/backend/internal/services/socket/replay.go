package socket

import (
	"container/list"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	FastUpdateInterval = 30 * time.Millisecond
	SlowUpdateInterval = 1000 * time.Millisecond
	BaseBuffer         = 10000 // * time.Millisecond
	MarketOpenHour     = 9
	MarketOpenMinute   = 30
	MarketCloseHour    = 16
	MarketCloseMinute  = 0
	ExtendedOpenHour   = 4
	ExtendedCloseHour  = 20
)

/*
//const tradeConditionsToCheck = new Set([2, 5, 7, 10, 12, 13, 15, 16, 20, 21, 22, 29, 33, 37, 52, 53])
const tradeConditionsToCheck = new Set([2, 5, 7, 10, 13, 15, 16, 20, 21, 22, 29, 33, 37, 52, 53])
const tradeConditionsToCheckVolume = new Set([15, 16, 38])
*/
func (c *Client) subscribeReplay(channelName string) {
	securityID, baseDataType, channelType, err := getInfoFromChannelName(channelName)
	if err != nil {
		////fmt.Println("Error getting info from channel name:", err)
		return
	}
	securityIDInt, err := strconv.Atoi(securityID)
	if err != nil {
		////fmt.Printf("do021 %v\n", err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s-%s", securityID, baseDataType)

	if _, exists := c.replayData[key]; !exists {
		c.replayData[key] = &ReplayData{
			baseDataType: baseDataType,
			channelTypes: []string{},
			data:         list.New(),
			refilling:    false,
			securityID:   securityIDInt,
		}
	}
	for _, existingChannelType := range c.replayData[key].channelTypes {
		if existingChannelType == channelType {
			return //this triggers if the channel is already subscribed, but we still want initial value for the new subscription???
		}
	}
	c.replayData[key].channelTypes = append(c.replayData[key].channelTypes, channelType)
	go func() {
		initialValue, err := getInitialStreamValue(c.conn, channelName, c.simulatedTime)
		if err != nil {
			////fmt.Println("Error fetching initial value for replay:", err)
			return
		}

		// Send to the client via the send channel (thread-safe)
		select {
		case c.send <- []byte(initialValue):
			// Successfully sent
		default:
			// Channel is full or closed, skip this message
		}
	}()

	if !c.loopRunning {
		c.loopRunning = true
		c.StartLoop() // Use go to start the loop in a separate goroutine
	}
}

func getInfoFromChannelName(channelName string) (string, string, string, error) {
	splits := strings.Split(channelName, "-")

	// e.g.: "123-slow-regular" => splits = [ "123", "slow", "regular" ]
	securityID := splits[0]

	if len(splits) == 2 {
		// might be just "123-slow" or "123-fast" or "123-all"
		switch splits[1] {
		case "all", "slow", "fast":
			return securityID, "trade", splits[1], nil
		case "quote":
			return securityID, "quote", "quote", nil
		case "close":
			return securityID, "close", "close", nil
		default:
			return "", "", "", fmt.Errorf("invalid channel type: %s", splits[1])
		}
	} else if len(splits) == 3 {
		// might be "slow-regular" or "slow-extended" or "fast-regular", etc.
		base := splits[1]  // "slow" or "fast"
		extra := splits[2] // "regular" or "extended"

		// combine them
		channelType := base + "-" + extra

		// Then decide baseDataType
		switch base {
		case "all", "slow", "fast":
			return securityID, "trade", channelType, nil
		case "quote":
			return securityID, "quote", channelType, nil
		case "close":
			return securityID, "close", channelType, nil
		}
		return "", "", "", fmt.Errorf("invalid channel type: %s", channelType)
	}

	return "", "", "", fmt.Errorf("invalid channelName format: %s", channelName)
}

func (c *Client) unsubscribeReplay(channelName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Determine the base data type
	securityID, baseDataType, channelType, err := getInfoFromChannelName(channelName)
	if err != nil {
		////fmt.Println("Error getting info from channel name:", err)
		return
	}
	key := fmt.Sprintf("%s-%s", securityID, baseDataType)
	if replayData, exists := c.replayData[key]; exists {
		for i, existingChannelType := range replayData.channelTypes {
			if existingChannelType == channelType {
				replayData.channelTypes[i] = replayData.channelTypes[len(replayData.channelTypes)-1]
				replayData.channelTypes = replayData.channelTypes[:len(replayData.channelTypes)-1]
				break
			}
		}
		if len(replayData.channelTypes) == 0 {
			delete(c.replayData, key)
		}
	}
}

func (c *Client) StartLoop() {
	go func() {
		ticker := time.NewTicker(FastUpdateInterval)
		defer ticker.Stop()
		lastSlow := time.Now()
		lastTimestampUpdate := time.Now()
		for range ticker.C {
			c.mu.Lock()
			now := time.Now()
			if c.replayActive && !c.replayPaused {
				delta := now.Sub(c.lastTickTime)
				c.lastTickTime = now
				c.accumulatedActiveTime += delta
				simulatedElapsed := time.Duration(float64(c.accumulatedActiveTime) * c.replaySpeed)
				c.simulatedTime = c.simulatedTimeStart + int64(simulatedElapsed/time.Millisecond)
				if now.Sub(lastTimestampUpdate) >= TimestampUpdateInterval {

					timestampUpdate := map[string]interface{}{
						"channel":   "timestamp",
						"timestamp": c.simulatedTime,
					}
					jsonData, err := json.Marshal(timestampUpdate)
					if err == nil {
						c.send <- jsonData
					}
					lastTimestampUpdate = now
				}
				if !c.isMarketOpen(c.simulatedTime) {
					c.jumpToNextMarketOpen()
					c.simulatedTimeStart = c.simulatedTime
					c.accumulatedActiveTime = 0
				}
				for _, replayData := range c.replayData {
					ticksToPush := make([]TickData, 0)
					for e := replayData.data.Front(); e != nil; {
						tick := e.Value.(TickData)
						next := e.Next()
						if c.simulatedTime >= tick.GetTimestamp() {
							ticksToPush = append(ticksToPush, tick)
							replayData.data.Remove(e)
						} else {
							break
						}
						e = next
					}
					if len(ticksToPush) > 0 {
						for _, channelType := range replayData.channelTypes {
							switch channelType {
							case "all", "close":
								for _, tick := range ticksToPush {
									select {
									case c.send <- jsonMarshalTick(tick, replayData.securityID, channelType):
									default:
										////fmt.Println("Warning: Failed to send data. Channel might be closed.")
									}
								}
							case "slow-regular", "slow-extended":
								if now.Sub(lastSlow) < SlowUpdateInterval {
									continue
								}
								lastSlow = now
								fallthrough
							case "fast-regular", "fast-extended":
								fallthrough
							case "quote":
								c.send <- jsonMarshalTick(aggregateTicks(ticksToPush, replayData.baseDataType), replayData.securityID, channelType)
							}
						}
					}
					if replayData.data.Back() == nil || c.simulatedTime >= replayData.data.Back().Value.(TickData).GetTimestamp()-c.buffer {
						if !replayData.refilling {
							replayData.refilling = true
							go c.fetchMoreData(replayData)
						}
					}
				}
			}
			c.mu.Unlock()
		}
	}()
}

// isMarketOpen checks if the current simulated time is within market hours.
func (c *Client) isMarketOpen(simulatedTime int64) bool {
	location, _ := time.LoadLocation("America/New_York")
	currentTime := time.Unix(simulatedTime/1000, 0).In(location) // Convert milliseconds to time.Time in New York timezone
	openHour, openMinute, closeHour, closeMinute := MarketOpenHour, MarketOpenMinute, MarketCloseHour, MarketCloseMinute
	if c.replayExtendedHours {
		openHour, openMinute, closeHour = ExtendedOpenHour, 0, ExtendedCloseHour
	}
	marketOpen := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), openHour, openMinute, 0, 0, location)
	marketClose := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), closeHour, closeMinute, 0, 0, location)
	return !currentTime.Before(marketOpen) && currentTime.Before(marketClose)
}

func (c *Client) jumpToNextMarketOpen() {

	location, _ := time.LoadLocation("America/New_York")
	simulatedTime := time.Unix(c.simulatedTime/1000, 0).In(location) // Convert milliseconds to time.Time in New York timezone
	simulatedTime = simulatedTime.Add(24 * time.Hour)
	////fmt.Printf("\n %v %v", simulatedTime, simulatedTime.Weekday())
	if simulatedTime.Weekday() == time.Saturday {
		simulatedTime = simulatedTime.Add(48 * time.Hour) // Skip to Monday
	} else if simulatedTime.Weekday() == time.Sunday {
		simulatedTime = simulatedTime.Add(24 * time.Hour) // Skip to Monday
	}
	openHour, openMinute := MarketOpenHour, MarketOpenMinute
	if c.replayExtendedHours {
		openHour, openMinute = ExtendedOpenHour, 0
	}
	nextMarketOpen := time.Date(simulatedTime.Year(), simulatedTime.Month(), simulatedTime.Day(), openHour, openMinute, 0, 0, location)
	c.simulatedTime = nextMarketOpen.Unix() * 1000 // Convert back to milliseconds
	c.simulatedTimeStart = c.simulatedTime
	c.accumulatedActiveTime = 0
	c.lastTickTime = time.Now()
}

func jsonMarshalTick(tick TickData, securityID int, channelType string) []byte {
	tick.SetChannel(fmt.Sprintf("%d-%s", securityID, channelType))
	data, err := json.Marshal(tick)
	if err != nil {
		////fmt.Println("Error marshaling tick:", err)
		return nil
	}
	return data
}
func (c *Client) fetchMoreData(replayData *ReplayData) {
	var newTicks []TickData
	var err error
	var timestamp int64
	if replayData.data == nil || replayData.data.Back() == nil {
		timestamp = c.simulatedTime
	} else {
		tick, ok := replayData.data.Back().Value.(TickData)
		if !ok {
			////fmt.Println("ERR ------- type assertion")
			return
		}
		timestamp = tick.GetTimestamp()
	}
	//////fmt.Println(timestamp,replayData.baseDataType, "---------------")
	switch replayData.baseDataType {

	case "trade":
		newTicks, err = getTradeData(c.conn, replayData.securityID, timestamp, c.buffer, c.replayExtendedHours)
	case "quote":
		newTicks, err = getQuoteData(c.conn, replayData.securityID, timestamp, c.buffer, c.replayExtendedHours)
	case "close":
		newTicks, err = getPrevCloseData(c.conn, replayData.securityID, timestamp)
	default:
		////fmt.Println("kn0-------------------")
		return
	}
	//////fmt.Println(len(newTicks))
	if err != nil {
		////fmt.Printf("idohn02io2 %v\n", err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tick := range newTicks {
		replayData.data.PushBack(tick)
	}
	replayData.refilling = false
}

func (c *Client) LoadTicks(ticker string, _ []string, ticks []TickData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, tick := range ticks {
		c.replayData[ticker].data.PushBack(tick)
	}
}

func (c *Client) playReplay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.replayPaused {
		c.replayActive = true
		c.replayPaused = false
		c.lastTickTime = time.Now()
	}
}

func (c *Client) pauseReplay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.replayPaused {
		c.replayPaused = true
	}
}

func (c *Client) stopReplay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replayActive = false
	c.replayPaused = false
	c.simulatedTime = 0
	c.replayData = make(map[string]*ReplayData)
	////fmt.Println("------closing----")
	select {
	case <-c.send:
	default:
	}
}

func (c *Client) setReplaySpeed(speed float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replaySpeed = speed
}

// /replay.go
