package agent

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
)

// PersistentContextItem represents a single piece of data stored in the persistent context.
type PersistentContextItem struct {
	Key       string          `json:"key"`        // Unique identifier for the context item
	Value     json.RawMessage `json:"value"`      // The actual data, stored as raw JSON
	Timestamp time.Time       `json:"timestamp"`  // When the item was last updated
	ExpiresAt time.Time       `json:"expires_at"` // Optional: When this specific item should expire (zero time means no specific expiration)
}

// PersistentContextData holds all persistent context items for a user.
type PersistentContextData struct {
	Items     map[string]PersistentContextItem `json:"items"`     // Map for efficient key-based access
	Timestamp time.Time                        `json:"timestamp"` // Last update time of the entire context set
}

// --- Core Cache Functions ---

const persistentContextKeyFormat = "user:%d:persistent_context"
const defaultPersistentContextExpiration = 7 * 24 * time.Hour // Default expiration for the whole set
const maxPersistentContextItems = 20                          // Max number of items to keep (pruning)

// savePersistentContext saves the entire persistent context data block to Redis.
func savePersistentContext(ctx context.Context, conn *data.Conn, userID int, data *PersistentContextData) error {
	if data == nil {
		return fmt.Errorf("cannot save nil persistent context data")
	}
	cacheKey := fmt.Sprintf(persistentContextKeyFormat, userID)

	// --- Pruning Logic --- Implement before saving
	now := time.Now()
	validItems := make(map[string]PersistentContextItem)
	for key, item := range data.Items {
		// Remove items with specific expiration dates that have passed
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			////fmt.Printf("Pruning expired persistent context item '%s' for user %d\n", key, userID)
			continue // Skip expired item
		}
		validItems[key] = item
	}
	data.Items = validItems

	// Prune by count if necessary (remove oldest items first)
	if len(data.Items) > maxPersistentContextItems {
		// Convert map to slice for sorting
		itemsSlice := make([]PersistentContextItem, 0, len(data.Items))
		for _, item := range data.Items {
			itemsSlice = append(itemsSlice, item)
		}

		// Sort by timestamp (oldest first)
		sort.Slice(itemsSlice, func(i, j int) bool {
			return itemsSlice[i].Timestamp.Before(itemsSlice[j].Timestamp)
		})

		// Keep only the newest 'maxPersistentContextItems'
		itemsToKeep := itemsSlice[len(itemsSlice)-maxPersistentContextItems:]

		// Rebuild the map with only the items to keep
		prunedItems := make(map[string]PersistentContextItem)
		for _, item := range itemsToKeep {
			prunedItems[item.Key] = item
		}
		data.Items = prunedItems
		////fmt.Printf("Pruned persistent context items for user %d to newest %d\n", userID, maxPersistentContextItems)
	}
	// --- End Pruning Logic ---

	data.Timestamp = time.Now() // Update last modified time

	serializedData, err := json.Marshal(data)
	if err != nil {
		////fmt.Printf("Failed to serialize persistent context for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to serialize persistent context: %w", err)
	}

	////fmt.Printf("Saving %d persistent context items for user %d to cache key: %s\n", len(data.Items), userID, cacheKey)
	err = conn.Cache.Set(ctx, cacheKey, serializedData, defaultPersistentContextExpiration).Err()
	if err != nil {
		////fmt.Printf("Failed to save persistent context to Redis for user %d: %v\n", userID, err)
		return fmt.Errorf("failed to save persistent context to cache: %w", err)
	}

	////fmt.Printf("Successfully saved persistent context for user %d to Redis.\n", userID)
	return nil
}

// getPersistentContext retrieves the persistent context data block from Redis.
func getPersistentContext(ctx context.Context, conn *data.Conn, userID int) (*PersistentContextData, error) {
	cacheKey := fmt.Sprintf(persistentContextKeyFormat, userID)

	cachedValue, err := conn.Cache.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss is not an error, just return an empty structure
			return &PersistentContextData{Items: make(map[string]PersistentContextItem), Timestamp: time.Time{}}, nil
		}
		////fmt.Printf("Error retrieving persistent context from Redis for user %d: %v\n", userID, err)
		return nil, fmt.Errorf("failed to retrieve persistent context from cache: %w", err)
	}

	var data PersistentContextData
	if err := json.Unmarshal([]byte(cachedValue), &data); err != nil {
		////fmt.Printf("Failed to deserialize persistent context for user %d: %v\n", userID, err)
		// If deserialization fails, return an empty structure to avoid breaking flows
		return &PersistentContextData{Items: make(map[string]PersistentContextItem), Timestamp: time.Time{}}, nil // Consider logging the error more prominently
	}

	// Ensure Items map is initialized if it was nil after unmarshalling (e.g., from empty JSON `"{}"`)
	if data.Items == nil {
		data.Items = make(map[string]PersistentContextItem)
	}

	// Optional: Filter out expired items during retrieval as well, although savePersistentContext should handle it.
	// This ensures consumers always get non-expired items even if pruning during save failed.
	now := time.Now()
	validItems := make(map[string]PersistentContextItem)
	needsResave := false
	for key, item := range data.Items {
		if !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now) {
			////fmt.Printf("Filtering expired persistent context item '%s' during retrieval for user %d\n", key, userID)
			needsResave = true
			continue
		}
		validItems[key] = item
	}

	// If items were filtered, update the data and potentially save back
	if needsResave {
		data.Items = validItems
		// Optional: Save the cleaned data back asynchronously
		//go func() {
			bgCtx := context.Background()
			if err := savePersistentContext(bgCtx, conn, userID, &data); err != nil {
                return nil, err
				////fmt.Printf("Error saving persistent context after filtering expired items during get for user %d: %v\n", userID, err)
			}
		//}()
	}

	////fmt.Printf("Retrieved %d persistent context items from cache for user %d.\n", len(data.Items), userID)
	return &data, nil
}

// --- Helper Functions for Modifying Context ---

// AddOrUpdatePersistentContextItem adds or updates a single item in the persistent context.
func AddOrUpdatePersistentContextItem(ctx context.Context, conn *data.Conn, userID int, key string, value interface{}, itemExpiration time.Duration) error {
	// 1. Get current context
	data, err := getPersistentContext(ctx, conn, userID)
	if err != nil {
		return fmt.Errorf("failed to get persistent context before update for key '%s': %w", key, err)
	}

	// Ensure Items map is initialized
	if data.Items == nil {
		data.Items = make(map[string]PersistentContextItem)
	}

	// 2. Marshal the new value to json.RawMessage
	rawValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for persistent context key '%s': %w", key, err)
	}

	// 3. Create/Update the item
	now := time.Now()
	var expiresAt time.Time
	if itemExpiration > 0 {
		expiresAt = now.Add(itemExpiration)
	}

	data.Items[key] = PersistentContextItem{
		Key:       key,
		Value:     rawValue,
		Timestamp: now,
		ExpiresAt: expiresAt, // Set specific expiration if provided
	}

	// 4. Save the updated context
	if err := savePersistentContext(ctx, conn, userID, data); err != nil {
		return fmt.Errorf("failed to save persistent context after update for key '%s': %w", key, err)
	}

	////fmt.Printf("Successfully added/updated persistent context item '%s' for user %d\n", key, userID)
	return nil
}

// RemovePersistentContextItem removes a specific item from the persistent context by its key.
func RemovePersistentContextItem(ctx context.Context, conn *data.Conn, userID int, key string) error {
	// 1. Get current context
	data, err := getPersistentContext(ctx, conn, userID)
	if err != nil {
		// If context doesn't exist, the item isn't there anyway
		if err.Error() == "redis: nil" || data == nil {
			return nil
		}
		return fmt.Errorf("failed to get persistent context before removing key '%s': %w", key, err)
	}

	// 2. Check if item exists and remove it
	if _, exists := data.Items[key]; exists {
		delete(data.Items, key)
		////fmt.Printf("Removed persistent context item '%s' for user %d\n", key, userID)

		// 3. Save the updated context
		if err := savePersistentContext(ctx, conn, userID, data); err != nil {
			return fmt.Errorf("failed to save persistent context after removing key '%s': %w", key, err)
		}
	} 
    //else {
		////fmt.Printf("Persistent context item '%s' not found for removal for user %d\n", key, userID)
		// Not an error if the item wasn't there
	//}

	return nil
}
