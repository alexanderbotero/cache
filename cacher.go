package cache

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"golang.org/x/sync/singleflight"
)

type store struct {
	data  map[reflect.Type]map[any]any
	mu    sync.RWMutex
	group singleflight.Group
}

var cacheStore = &store{
	data: make(map[reflect.Type]map[any]any),
}

// Get retrieves a value from cache or computes it using getterFunc.
// It is thread-safe and handles concurrent access correctly.
// Errors from getterFunc are not cached, allowing retries.
//
// Returns an error if:
//   - getterFunc is nil
//   - getterFunc returns an error
//   - cache corruption is detected
func Get[K comparable, V any](key K, getterFunc func(K) (V, error)) (V, error) {
	var zero V
	if getterFunc == nil {
		return zero, errors.New("getterFunc cannot be nil")
	}
	// Get type safely
	valueType := getTypeOf(zero)

	// Fast path: check if already cached
	cacheStore.mu.RLock()
	storedValue, keyExists := cacheStore.data[valueType][key]
	if keyExists {
		cacheStore.mu.RUnlock()
		// Safe type assertion
		if typedValue, ok := storedValue.(V); ok {
			return typedValue, nil
		}
		// This case indicates cache corruption (internal bug)
		return zero, errors.New("cache corruption: stored value type mismatch")
	}
	cacheStore.mu.RUnlock()

	// Ensure the type exists
	ensureType(valueType)

	// Create a unique singleflight key that combines type + key
	// This ensures that different types don't collide
	sfKey := fmt.Sprintf("%v:%v", valueType, key)

	// Use singleflight to deduplicate concurrent calls
	result, err, _ := cacheStore.group.Do(sfKey, func() (any, error) {
		// Double-check: another goroutine might have cached while we were waiting
		cacheStore.mu.RLock()
		if storedValue, exists := cacheStore.data[valueType][key]; exists {
			cacheStore.mu.RUnlock()
			return storedValue, nil
		}
		cacheStore.mu.RUnlock()

		// Execute the getter (only ONE goroutine reaches here)
		uncached, err := getterFunc(key)
		if err != nil {
			return nil, fmt.Errorf("cache getter failed for key %v: %w", key, err)
		}

		// Cache the result
		cacheStore.mu.Lock()
		typeMapLocked := cacheStore.data[valueType]
		typeMapLocked[key] = uncached
		cacheStore.mu.Unlock()

		return uncached, nil
	})

	if err != nil {
		return zero, err
	}

	// Final type assertion
	typedValue, ok := result.(V)
	if !ok {
		return zero, errors.New("cache corruption: stored value type mismatch")
	}

	return typedValue, nil
}

func getTypeOf[T any](zero T) reflect.Type {
	typ := reflect.TypeOf(zero)
	// If nil (interfaces or pointers), use alternative method
	if typ == nil {
		typ = reflect.TypeOf((*T)(nil)).Elem()
	}
	return typ
}

func ensureType(valueType reflect.Type) {
	// First check: fast read with RLock
	cacheStore.mu.RLock()
	_, ok := cacheStore.data[valueType]
	cacheStore.mu.RUnlock()

	// If it already exists, return immediately
	if ok {
		return
	}

	// Second check: with Lock to avoid race condition
	cacheStore.mu.Lock()
	defer cacheStore.mu.Unlock()
	// Check again in case another goroutine created it while we were waiting for the Lock
	if _, ok := cacheStore.data[valueType]; !ok {
		cacheStore.data[valueType] = make(map[any]any)
	}
}
