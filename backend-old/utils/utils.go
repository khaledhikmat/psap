package utils

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	PodNameEnvKey      = "MY_POD_NAME"
	InstanceTypeEnvKey = "INSTANCE_TYPE"
)

var letters = []rune("0123456789ABCDEFGHIJKLMNOPQRTSUVMXYZ")
var numbers = []rune("0123456789")

type AtomicSlice[T any] struct {
	slice []T
	mu    sync.RWMutex
}

func (a *AtomicSlice[T]) Append(value T) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.slice = append(a.slice, value)
	return len(a.slice) - 1
}

func (a *AtomicSlice[T]) Get(index int) (T, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if index < 0 || index >= len(a.slice) {
		var zero T
		return zero, false
	}

	return a.slice[index], true
}

func (a *AtomicSlice[T]) Remove(index int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if index < 0 || index >= len(a.slice) {
		return false
	}

	a.slice = append(a.slice[:index], a.slice[index+1:]...)
	return true
}

func (a *AtomicSlice[T]) Length() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.slice)
}

func (a *AtomicSlice[T]) Iterate(action func(int, T)) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for idx, value := range a.slice {
		action(idx, value)
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func GetInstanceType() string {
	typ := os.Getenv(InstanceTypeEnvKey)
	if typ == "" {
		typ = "UNKNOWN"
	}

	return typ
}

func GetPodName() string {
	pod := os.Getenv(PodNameEnvKey)
	if pod == "" {
		pod = RandString(20)
	}

	return pod
}

func Convert2EventAttrs(otelAttrs []attribute.KeyValue) map[string]interface{} {
	eventAttrs := make(map[string]interface{}, len(otelAttrs))
	for _, attr := range otelAttrs {
		// This assumes all attribute values can be represented as strings.
		eventAttrs[string(attr.Key)] = attr.Value.AsString()
	}
	return eventAttrs
}

func RandNumber(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = numbers[rand.Intn(len(numbers))]
	}

	return string(b)
}

func RandString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

func Contains[T comparable](slice []T, item T) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func CreateDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func ToEpoch(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func FromEpoch(epochMillis int64) time.Time {
	return time.Unix(0, epochMillis*int64(time.Millisecond))
}

func ChunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	if chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}

	return append(chunks, items)
}

func AbsDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}

	return d
}

func ConditionString(s string) string {
	// Replace all spaces with underscores
	sWithoutSpaces := strings.ReplaceAll(s, " ", "_")
	// Convert the resulting string to lowercase
	return strings.ToLower(sWithoutSpaces)
}

func TimeToLive() int64 {
	hrs := 24 // Default to 24 hrs
	var ttl = os.Getenv("TIME_TO_LIVE_HRS")
	if ttl != "" {
		if intVal, err := strconv.Atoi(ttl); err == nil {
			hrs = intVal
		}
	}

	return time.Now().Add(time.Hour * time.Duration(hrs)).Unix()
}

// GetEnvInt retrieves an integer environment variable with fallback default
func GetEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func GetEnvInt(key string, defaultValue int) int {
	return GetEnvIntWithDefault(key, defaultValue)
}

func GetEnvStringWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetEnvString(key string, defaultValue string) string {
	return GetEnvStringWithDefault(key, defaultValue)
}

func GetEnvWithDefault(key, defaultValue string) string {
	return GetEnvStringWithDefault(key, defaultValue)
}

func GetEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func GetEnvStringSliceWithDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
