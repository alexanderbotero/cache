package cache

import (
	"errors"
	"reflect"
	"sync"
)

type store struct {
	data map[reflect.Type]map[any]any
	mu   sync.RWMutex
}

var cacheStore = &store{
	data: make(map[reflect.Type]map[any]any),
}

// Cache retrieves a value from cache or computes it using getterFunc.
// It is thread-safe and handles concurrent access correctly.
// Errors from getterFunc are not cached, allowing retries.
//
// Returns an error if:
//   - getterFunc is nil
//   - getterFunc returns an error
//   - cache corruption is detected
func Cache[K comparable, V any](key K, getterFunc func(K) (V, error)) (V, error) {
	var zero V
	if getterFunc == nil {
		return zero, errors.New("getterFunc cannot be nil")
	}
	// Get type safely
	valueType := getTypeOf(zero)
	// If there's a cached value, return it
	cacheStore.mu.RLock()
	storedValue, keyExists := cacheStore.data[valueType][key]
	if keyExists {
		cacheStore.mu.RUnlock() // Release lock before processing
		// Safe type assertion
		if typedValue, ok := storedValue.(V); ok {
			return typedValue, nil
		}
		// This case indicates cache corruption (internal bug)
		return zero, errors.New("cache corruption: stored value type mismatch")
	}
	cacheStore.mu.RUnlock() // Release lock if we didn't find the value
	// Ensure the type exists
	ensureType(valueType)
	uncached, err := getterFunc(key)
	if err != nil {
		return zero, err
	}
	cacheStore.mu.Lock()
	defer cacheStore.mu.Unlock()

	// Double-check: another goroutine might have cached the value while we were calling getterFunc
	typeMapLocked := cacheStore.data[valueType] // ensureType() guarantees this exists
	cachedValue, found := typeMapLocked[key]
	if !found {
		// Key not cached yet, cache our value
		typeMapLocked[key] = uncached
		return uncached, nil
	}

	// Type assertion on the value that was cached by another goroutine
	typedValue, ok := cachedValue.(V)
	if !ok {
		// Corruption case (should never happen)
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
