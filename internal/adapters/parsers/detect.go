package parsers

import (
	"bytes"
	"encoding/json"
)

// hasJSONKeys reports whether the JSON document in sample declares every key
// in want at any object level. It tolerates truncated input (the detection
// sample is only the head+tail of a large file) by stopping at the first
// decode error and using the keys seen so far.
//
// Distinguishing formats by *declared keys* rather than substring presence
// prevents false positives from string values that merely mention a keyword
// (e.g. a webpack module name containing "inputs").
func hasJSONKeys(sample []byte, want ...string) bool {
	keys := make(map[string]bool, len(want))
	dec := json.NewDecoder(bytes.NewReader(sample))

	type frame struct{ isObject bool }
	var stack []frame
	var pendingKey string
	expectKey := false

	record := func() {
		if pendingKey != "" {
			keys[pendingKey] = true
			pendingKey = ""
		}
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				record()
				stack = append(stack, frame{isObject: true})
				expectKey = true
			case '[':
				record()
				stack = append(stack, frame{isObject: false})
				expectKey = false
			case '}':
				record()
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				expectKey = len(stack) > 0 && stack[len(stack)-1].isObject
			case ']':
				record()
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				expectKey = len(stack) > 0 && stack[len(stack)-1].isObject
			}
		case string:
			if expectKey {
				pendingKey = t
				expectKey = false
			} else {
				record()
				expectKey = len(stack) > 0 && stack[len(stack)-1].isObject
			}
		case json.Number, bool, nil:
			record()
			expectKey = len(stack) > 0 && stack[len(stack)-1].isObject
		}
	}

	for _, w := range want {
		if !keys[w] {
			return false
		}
	}
	return true
}