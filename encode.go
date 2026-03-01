package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Encode encodes a Go value into a bencoded string.
// Supported types:
//   - int, int8, int16, int32, int64  → i<n>e
//   - uint, uint8, uint16, uint32, uint64 → i<n>e
//   - string  → <len>:<data>
//   - []any   → l<items>e
//   - map[string]any → d<key><value>...e  (keys sorted lexicographically)
func Encode(v any) (string, error) {
	var b strings.Builder
	if err := encode(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func encode(b *strings.Builder, v any) error {
	switch val := v.(type) {
	case int:
		return encodeInt(b, int64(val))
	case int8:
		return encodeInt(b, int64(val))
	case int16:
		return encodeInt(b, int64(val))
	case int32:
		return encodeInt(b, int64(val))
	case int64:
		return encodeInt(b, val)
	case uint:
		return encodeInt(b, int64(val))
	case uint8:
		return encodeInt(b, int64(val))
	case uint16:
		return encodeInt(b, int64(val))
	case uint32:
		return encodeInt(b, int64(val))
	case uint64:
		return encodeInt(b, int64(val))
	case string:
		return encodeString(b, val)
	case []any:
		return encodeList(b, val)
	case map[string]any:
		return encodeDict(b, val)
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
}

func encodeInt(b *strings.Builder, n int64) error {
	b.WriteByte('i')
	b.WriteString(strconv.FormatInt(n, 10))
	b.WriteByte('e')
	return nil
}

func encodeString(b *strings.Builder, s string) error {
	b.WriteString(strconv.Itoa(len(s)))
	b.WriteByte(':')
	b.WriteString(s)
	return nil
}

func encodeList(b *strings.Builder, list []any) error {
	b.WriteByte('l')
	for _, item := range list {
		if err := encode(b, item); err != nil {
			return err
		}
	}
	b.WriteByte('e')
	return nil
}

func encodeDict(b *strings.Builder, dict map[string]any) error {
	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteByte('d')
	for _, k := range keys {
		if err := encodeString(b, k); err != nil {
			return err
		}
		if err := encode(b, dict[k]); err != nil {
			return fmt.Errorf("dict value for key %q: %w", k, err)
		}
	}
	b.WriteByte('e')
	return nil
}
