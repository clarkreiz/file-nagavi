package main

import (
	"fmt"
	"strconv"
)

// Decode decodes a bencoded string and returns the Go value.
// Supported types:
//   - integer: i<n>e  → int64
//   - string:  <len>:<data>  → string
//   - list:    l<items>e  → []any
//   - dict:    d<key><value>...e  → map[string]any (keys are always strings)
func Decode(data string) (any, error) {
	val, _, err := decode(data, 0)
	return val, err
}

func decode(data string, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unexpected end of data at position %d", pos)
	}

	switch {
	case data[pos] == 'i':
		return decodeInt(data, pos)
	case data[pos] == 'l':
		return decodeList(data, pos)
	case data[pos] == 'd':
		return decodeDict(data, pos)
	case data[pos] >= '0' && data[pos] <= '9':
		return decodeString(data, pos)
	default:
		return nil, pos, fmt.Errorf("unknown type marker %q at position %d", data[pos], pos)
	}
}

// decodeInt parses i<integer>e
func decodeInt(data string, pos int) (int64, int, error) {
	pos++ // skip 'i'
	end := pos
	for end < len(data) && data[end] != 'e' {
		end++
	}
	if end >= len(data) {
		return 0, pos, fmt.Errorf("unterminated integer at position %d", pos)
	}
	raw := data[pos:end]
	// Reject invalid forms: i-0e, i03e, etc.
	if raw == "-0" {
		return 0, pos, fmt.Errorf("invalid integer -0")
	}
	if len(raw) > 1 && raw[0] == '0' {
		return 0, pos, fmt.Errorf("invalid integer with leading zero: %q", raw)
	}
	if len(raw) > 2 && raw[0] == '-' && raw[1] == '0' {
		return 0, pos, fmt.Errorf("invalid integer with leading zero: %q", raw)
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid integer %q: %w", raw, err)
	}
	return n, end + 1, nil // +1 to skip 'e'
}

// decodeString parses <length>:<data>
func decodeString(data string, pos int) (string, int, error) {
	colon := pos
	for colon < len(data) && data[colon] != ':' {
		colon++
	}
	if colon >= len(data) {
		return "", pos, fmt.Errorf("missing colon in string at position %d", pos)
	}
	length, err := strconv.Atoi(data[pos:colon])
	if err != nil || length < 0 {
		return "", pos, fmt.Errorf("invalid string length at position %d", pos)
	}
	start := colon + 1
	end := start + length
	if end > len(data) {
		return "", pos, fmt.Errorf("string length %d exceeds data at position %d", length, pos)
	}
	return data[start:end], end, nil
}

// decodeList parses l<items>e
func decodeList(data string, pos int) ([]any, int, error) {
	pos++ // skip 'l'
	list := make([]any, 0)
	for pos < len(data) && data[pos] != 'e' {
		val, next, err := decode(data, pos)
		if err != nil {
			return nil, pos, err
		}
		list = append(list, val)
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated list")
	}
	return list, pos + 1, nil // +1 to skip 'e'
}

// decodeDict parses d<key><value>...e
func decodeDict(data string, pos int) (map[string]any, int, error) {
	pos++ // skip 'd'
	dict := make(map[string]any)
	for pos < len(data) && data[pos] != 'e' {
		key, next, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict key: %w", err)
		}
		pos = next

		val, next, err := decode(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict value for key %q: %w", key, err)
		}
		dict[key] = val
		pos = next
	}
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated dict")
	}
	return dict, pos + 1, nil // +1 to skip 'e'
}
