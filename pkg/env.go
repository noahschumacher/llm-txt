package pkg

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type envParser[T any] func(string) (T, error)

func loadEnv[T any](key string, required bool, parser envParser[T]) T {
	val := getEnv(key, required)
	if val == "" {
		var zero T
		return zero
	}
	result, err := parser(val)
	if err != nil {
		log.Fatalf("Failed to parse %s: %v", key, err)
	}
	return result
}

func getEnv(key string, required bool) string {
	val := os.Getenv(key)
	if required && val == "" {
		log.Fatalf("%s is not set", key)
	}
	return val
}

func LoadStringEnv(key string, required bool) string {
	return getEnv(key, required)
}

func LoadIntEnv(key string, required bool) int {
	return loadEnv(key, required, strconv.Atoi)
}

func LoadBoolEnv(key string, required bool) bool {
	return loadEnv(key, required, strconv.ParseBool)
}

func LoadDurationEnv(key string, required bool) time.Duration {
	return loadEnv(key, required, time.ParseDuration)
}

func LoadStringSliceEnv(key string, required bool) []string {
	return loadEnv(key, required, func(val string) ([]string, error) {
		val = strings.ReplaceAll(val, " ", "")
		return strings.Split(val, ","), nil
	})
}
