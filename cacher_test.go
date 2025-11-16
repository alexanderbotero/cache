package cache

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type CacherTestSuite struct {
	suite.Suite
	callCount atomic.Int32
}

func TestCacherSuite(t *testing.T) {
	suite.Run(t, new(CacherTestSuite))
}

// SetupTest runs before each test
func (s *CacherTestSuite) SetupTest() {
	// Clean the cache before each test
	cacheStore.mu.Lock()
	cacheStore.data = make(map[reflect.Type]map[any]any)
	cacheStore.mu.Unlock()

	// Reset counter
	s.callCount.Store(0)
}

// TearDownTest runs after each test
func (s *CacherTestSuite) TearDownTest() {
	// Explicit cache cleanup
	cacheStore.mu.Lock()
	cacheStore.data = make(map[reflect.Type]map[any]any)
	cacheStore.mu.Unlock()
}

// TestCacheCallsGetterOnlyOnce verifies that the getter is called only once
func (s *CacherTestSuite) TestCacheCallsGetterOnlyOnce() {
	// Define a getter that increments a counter
	getter := func(key int) (string, error) {
		s.callCount.Add(1)
		return "cached value", nil
	}

	// First call - should call the getter
	result1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.Equal("cached value", result1)
	s.Equal(int32(1), s.callCount.Load(), "Getter should have been called once")

	// Multiple subsequent calls - none should call the getter
	for i := 0; i < 5; i++ {
		result, err := Cache(1, getter)
		s.NoError(err)
		s.Equal("cached value", result)
		s.Equal(int32(1), s.callCount.Load(), "Getter should only have been called once")
	}
}

// TestCacheCallsGetterForDifferentKeys verifies that it's called for different keys
func (s *CacherTestSuite) TestCacheCallsGetterForDifferentKeys() {
	getter := func(key int) (string, error) {
		s.callCount.Add(1)
		return "value", nil
	}

	// Three different keys should call the getter 3 times
	result1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.Equal("value", result1)
	s.Equal(int32(1), s.callCount.Load())

	result2, err2 := Cache(2, getter)
	s.NoError(err2)
	s.Equal("value", result2)
	s.Equal(int32(2), s.callCount.Load())

	result3, err3 := Cache(3, getter)
	s.NoError(err3)
	s.Equal("value", result3)
	s.Equal(int32(3), s.callCount.Load())

	// Call again with key 1 - should NOT increment
	result4, err4 := Cache(1, getter)
	s.NoError(err4)
	s.Equal("value", result4)
	s.Equal(int32(3), s.callCount.Load(), "Key 1 was already cached")
}

// TestCacheDifferentTypesHaveSeparateCache verifies that different types have separate caches
func (s *CacherTestSuite) TestCacheDifferentTypesHaveSeparateCache() {
	var stringCalls, intCalls atomic.Int32

	stringGetter := func(key int) (string, error) {
		stringCalls.Add(1)
		return "string value", nil
	}

	intGetter := func(key int) (int, error) {
		intCalls.Add(1)
		return 42, nil
	}

	// First call for each type
	strResult1, err1 := Cache(1, stringGetter)
	s.NoError(err1)
	s.Equal("string value", strResult1)
	s.Equal(int32(1), stringCalls.Load())

	intResult1, err2 := Cache(1, intGetter)
	s.NoError(err2)
	s.Equal(42, intResult1)
	s.Equal(int32(1), intCalls.Load())

	// Second call - none should call the getter
	strResult2, err3 := Cache(1, stringGetter)
	s.NoError(err3)
	s.Equal("string value", strResult2)
	s.Equal(int32(1), stringCalls.Load(), "String getter should NOT have been called again")

	intResult2, err4 := Cache(1, intGetter)
	s.NoError(err4)
	s.Equal(42, intResult2)
	s.Equal(int32(1), intCalls.Load(), "Int getter should NOT have been called again")
}

// TestCacheDoesNotCacheErrors verifies that errors are not cached
func (s *CacherTestSuite) TestCacheDoesNotCacheErrors() {
	var errorCount atomic.Int32

	getter := func(key int) (string, error) {
		s.callCount.Add(1)
		count := errorCount.Add(1)
		if count <= 2 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	}

	// First call - error
	_, err1 := Cache(1, getter)
	s.Error(err1)
	s.Equal(int32(1), s.callCount.Load())

	// Second call - error (called again because error was not cached)
	_, err2 := Cache(1, getter)
	s.Error(err2)
	s.Equal(int32(2), s.callCount.Load(), "Getter should be called again because there was an error")

	// Third call - success
	result, err3 := Cache(1, getter)
	s.NoError(err3)
	s.Equal("success", result)
	s.Equal(int32(3), s.callCount.Load())

	// Fourth call - should return from cache
	result2, err4 := Cache(1, getter)
	s.NoError(err4)
	s.Equal("success", result2)
	s.Equal(int32(3), s.callCount.Load(), "Getter should NOT be called because it's now cached")
}

// TestCacheWithNilGetterFunc verifies that it returns an error when getterFunc is nil
func (s *CacherTestSuite) TestCacheWithNilGetterFunc() {
	result, err := Cache[int, string](1, nil)
	s.Error(err)
	s.Equal("", result)
	s.Contains(err.Error(), "getterFunc cannot be nil")
}

// TestCacheWithPointerTypes verifies that it works with pointer types
func (s *CacherTestSuite) TestCacheWithPointerTypes() {
	type User struct {
		ID   int
		Name string
	}

	getter := func(id int) (*User, error) {
		s.callCount.Add(1)
		return &User{ID: id, Name: "Bob"}, nil
	}

	// First call
	user1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.NotNil(user1)
	s.Equal(1, user1.ID)
	s.Equal("Bob", user1.Name)
	s.Equal(int32(1), s.callCount.Load())

	// Second call - should return from cache
	user2, err2 := Cache(1, getter)
	s.NoError(err2)
	s.NotNil(user2)
	s.Equal(1, user2.ID)
	s.Equal("Bob", user2.Name)
	s.Equal(int32(1), s.callCount.Load(), "Getter should NOT have been called again")

	// Verify they are the same pointer (same cached object)
	s.Equal(user1, user2, "Should be the same pointer from cache")
}

// Reader interface for testing
type Reader interface {
	Read() string
}

// StringReader implements Reader
type StringReader struct {
	data string
}

func (sr *StringReader) Read() string {
	return sr.data
}

// TestCacheWithInterfaceTypes verifies that it works with interfaces
func (s *CacherTestSuite) TestCacheWithInterfaceTypes() {
	getter := func(id int) (Reader, error) {
		s.callCount.Add(1)
		return &StringReader{data: "interface value"}, nil
	}

	// First call
	reader1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.NotNil(reader1)
	s.Equal("interface value", reader1.Read())
	s.Equal(int32(1), s.callCount.Load())

	// Second call - should return from cache
	reader2, err2 := Cache(1, getter)
	s.NoError(err2)
	s.NotNil(reader2)
	s.Equal("interface value", reader2.Read())
	s.Equal(int32(1), s.callCount.Load(), "Getter should NOT have been called again")
}

// TestCacheWithPointerReturnType verifies getTypeOf with pointer types
func (s *CacherTestSuite) TestCacheWithPointerReturnType() {
	type Product struct {
		SKU string
	}

	// Getter that returns a pointer
	getter := func(id int) (*Product, error) {
		s.callCount.Add(1)
		return &Product{SKU: "ABC123"}, nil
	}

	// First call
	product1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.NotNil(product1)
	s.Equal("ABC123", product1.SKU)
	s.Equal(int32(1), s.callCount.Load())

	// Second call
	product2, err2 := Cache(1, getter)
	s.NoError(err2)
	s.NotNil(product2)
	s.Equal("ABC123", product2.SKU)
	s.Equal(int32(1), s.callCount.Load())
}

// TestCacheWithNilPointerReturned verifies that it can cache nil pointers
func (s *CacherTestSuite) TestCacheWithNilPointerReturned() {
	type Product struct {
		SKU string
	}

	nilGetter := func(id int) (*Product, error) {
		s.callCount.Add(1)
		return nil, nil // Return nil pointer explicitly
	}

	// First call
	product1, err1 := Cache(1, nilGetter)
	s.NoError(err1)
	s.Nil(product1, "Should be able to cache nil")
	s.Equal(int32(1), s.callCount.Load())

	// Second call - should return nil from cache
	product2, err2 := Cache(1, nilGetter)
	s.NoError(err2)
	s.Nil(product2, "Should return nil from cache")
	s.Equal(int32(1), s.callCount.Load(), "Should not call again")
}

// TestCacheCorruption simulates cache corruption (storing incorrect type)
func (s *CacherTestSuite) TestCacheCorruption() {
	// First cache a normal value
	getter := func(id int) (string, error) {
		return "correct value", nil
	}

	result1, err1 := Cache(1, getter)
	s.NoError(err1)
	s.Equal("correct value", result1)

	// NOTE: Direct access to internals to simulate corruption
	// In production code this should never happen
	var v string
	valueType := getTypeOf(v)
	cacheStore.mu.Lock()
	cacheStore.data[valueType][1] = 12345 // ❌ Intentional corruption: we store int instead of string
	cacheStore.mu.Unlock()

	// Try to retrieve - should detect corruption
	result2, err2 := Cache(1, getter)
	s.Error(err2)
	s.Contains(err2.Error(), "cache corruption")
	s.Equal("", result2) // Zero value of string
}

// TestConcurrentReadsAndWrites verifies concurrent operations
func (s *CacherTestSuite) TestConcurrentReadsAndWrites() {
	getter := func(key int) (string, error) {
		return fmt.Sprintf("value-%d", key), nil
	}

	var wg sync.WaitGroup

	// 100 goroutines reading and writing simultaneously
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Mix of reads of existing and new keys
			key := id % 10 // Only 10 different keys
			result, err := Cache(key, getter)
			s.NoError(err)
			s.Contains(result, "value-")
		}(i)
	}

	wg.Wait()

	// Verify that values are correctly cached
	for i := 0; i < 10; i++ {
		result, err := Cache(i, getter)
		s.NoError(err)
		s.Equal(fmt.Sprintf("value-%d", i), result)
	}
}

// TestConcurrentSameKey verifies multiple goroutines with the same key
func (s *CacherTestSuite) TestConcurrentSameKey() {
	callCount := atomic.Int32{}
	getter := func(key int) (string, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow operation
		return "value", nil
	}

	var wg sync.WaitGroup

	// 50 goroutines trying to cache the same key simultaneously
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Cache(1, getter)
			s.NoError(err)
			s.Equal("value", result)
		}()
	}

	wg.Wait()

	// With singleflight, the getter should be called exactly once
	s.Equal(int32(1), callCount.Load(),
		"With singleflight, getter should be called exactly once for concurrent requests")
}

// TestSingleflightMultipleKeys verifies that different keys don't block each other
func (s *CacherTestSuite) TestSingleflightMultipleKeys() {
	callCount := atomic.Int32{}
	getter := func(key int) (string, error) {
		callCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return fmt.Sprintf("value-%d", key), nil
	}

	var wg sync.WaitGroup

	// 10 goroutines per key (total 30 goroutines for 3 keys)
	for key := 1; key <= 3; key++ {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				result, err := Cache(k, getter)
				s.NoError(err)
				s.Equal(fmt.Sprintf("value-%d", k), result)
			}(key)
		}
	}

	wg.Wait()

	// Should be called exactly 3 times (once per unique key)
	s.Equal(int32(3), callCount.Load(),
		"Should be called once per unique key, even with multiple concurrent requests per key")
}
