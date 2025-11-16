# Cache

A thread-safe, type-safe generic cache for Go with zero dependencies.

## Features

- 🔒 **Thread-safe**: Safe for concurrent access with efficient read/write locking
- 🎯 **Type-safe**: Leverages Go generics for compile-time type safety
- 🚀 **Zero dependencies**: Only uses Go standard library (tests use testify)
- ⚡ **Efficient**: Double-check locking pattern minimizes lock contention
- 🔄 **Smart error handling**: Errors are not cached, allowing retries
- 🗂️ **Type partitioning**: Separate cache spaces per type automatically

## Requirements

- Go 1.18 or later

## Installation

```bash
go get github.com/alexanderbotero/cache
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/alexanderbotero/cache"
)

func main() {
    // Cache a simple value
    result, err := cache.Cache(1, func(id int) (string, error) {
        // This function is only called once per unique key
        return fmt.Sprintf("user-%d", id), nil
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // Output: user-1
    
    // Subsequent calls return the cached value instantly
    result2, _ := cache.Cache(1, func(id int) (string, error) {
        return "this won't be called", nil
    })
    fmt.Println(result2) // Output: user-1
}
```

## Usage Examples

### Basic Caching

```go
// Cache database queries
user, err := cache.Cache(userID, func(id int) (*User, error) {
    return db.GetUser(id)
})

// Cache API calls
data, err := cache.Cache("api-key", func(key string) ([]byte, error) {
    return http.Get("https://api.example.com/data")
})
```

### Different Types, Separate Caches

```go
// String cache
str, _ := cache.Cache(1, func(k int) (string, error) {
    return "hello", nil
})

// Int cache with same key - completely separate!
num, _ := cache.Cache(1, func(k int) (int, error) {
    return 42, nil
})
```

### Error Handling

```go
// Errors are NOT cached - retries are allowed
result, err := cache.Cache("key", func(k string) (string, error) {
    if networkIsDown() {
        return "", errors.New("network error") // Not cached
    }
    return "success", nil // This will be cached
})
```

### Pointer Types and Interfaces

```go
// Works with pointers
user, err := cache.Cache(1, func(id int) (*User, error) {
    return &User{ID: id, Name: "Alice"}, nil
})

// Works with interfaces
reader, err := cache.Cache("file.txt", func(path string) (io.Reader, error) {
    return os.Open(path)
})
```

## How It Works

1. **Type Partitioning**: The cache automatically separates data by type using `reflect.Type` as a key. This means `Cache[int, string]` and `Cache[int, int]` maintain separate cache spaces.

2. **Thread Safety**: Uses `sync.RWMutex` for efficient concurrent access:
   - Multiple goroutines can read simultaneously
   - Writes are exclusive
   - Double-check locking prevents unnecessary writes

3. **Getter Function**: The `getterFunc` is called only once per unique key (unless it returns an error). Subsequent calls return the cached value.

## API

### Cache

```go
func Cache[K comparable, V any](key K, getterFunc func(K) (V, error)) (V, error)
```

Retrieves a value from cache or computes it using `getterFunc`.

**Parameters:**
- `key`: The cache key (must be comparable)
- `getterFunc`: Function to generate the value if not cached (cannot be nil)

**Returns:**
- The cached or computed value
- An error if:
  - `getterFunc` is nil
  - `getterFunc` returns an error
  - Cache corruption is detected (internal bug)

**Thread-Safety:** This function is safe for concurrent use.

## Limitations

- No built-in eviction policy (cache grows indefinitely)
- No TTL (time-to-live) support
- No memory limits
- Global cache instance (all callers share the same cache)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

Alexander Botero ([@alexanderbotero](https://github.com/alexanderbotero))
