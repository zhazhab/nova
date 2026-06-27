package middleware

import (
	"strings"
	"unicode/utf8"
)

// jsonStringFieldCounter incrementally counts decoded characters inside one
// JSON string field while tool-call args are still streaming.
type jsonStringFieldCounter struct {
	field            string
	scanBuffer       string
	inValue          bool
	done             bool
	escaped          bool
	unicodeRemaining int
}

func newJSONStringFieldCounter(field string) jsonStringFieldCounter {
	return jsonStringFieldCounter{field: field}
}

func (c *jsonStringFieldCounter) Write(chunk string) int {
	if c == nil || c.done || chunk == "" {
		return 0
	}
	count := 0
	input := chunk
	for input != "" {
		if !c.inValue {
			c.scanBuffer += input
			offset, ok := jsonStringFieldValueOffset(c.scanBuffer, c.field)
			if !ok {
				c.trimScanBuffer()
				return count
			}
			input = c.scanBuffer[offset:]
			c.scanBuffer = ""
			c.inValue = true
			continue
		}
		consumed, delta, done := c.countJSONStringValueChunk(input)
		count += delta
		input = input[consumed:]
		if done {
			c.done = true
			c.inValue = false
			return count
		}
	}
	return count
}

func (c *jsonStringFieldCounter) trimScanBuffer() {
	const maxScanBuffer = 256
	if len(c.scanBuffer) > maxScanBuffer {
		c.scanBuffer = c.scanBuffer[len(c.scanBuffer)-maxScanBuffer:]
	}
}

func (c *jsonStringFieldCounter) countJSONStringValueChunk(value string) (int, int, bool) {
	consumed := 0
	count := 0
	for consumed < len(value) {
		if c.unicodeRemaining > 0 {
			remaining := len(value) - consumed
			if remaining >= c.unicodeRemaining {
				consumed += c.unicodeRemaining
				c.unicodeRemaining = 0
				count++
			} else {
				c.unicodeRemaining -= remaining
				consumed = len(value)
			}
			continue
		}
		ch := value[consumed]
		if c.escaped {
			c.escaped = false
			if ch == 'u' {
				c.unicodeRemaining = 4
				consumed++
				continue
			}
			count++
			consumed++
			continue
		}
		switch ch {
		case '\\':
			c.escaped = true
			consumed++
		case '"':
			consumed++
			return consumed, count, true
		default:
			_, size := utf8.DecodeRuneInString(value[consumed:])
			if size <= 0 {
				size = 1
			}
			count++
			consumed += size
		}
	}
	return consumed, count, false
}

func jsonStringFieldValueOffset(data, field string) (int, bool) {
	needle := `"` + field + `"`
	searchFrom := 0
	for {
		index := strings.Index(data[searchFrom:], needle)
		if index < 0 {
			return 0, false
		}
		index += searchFrom
		afterKey := strings.TrimLeft(data[index+len(needle):], " \n\r\t")
		if afterKey == "" {
			return 0, false
		}
		if !strings.HasPrefix(afterKey, ":") {
			searchFrom = index + len(needle)
			continue
		}
		afterColon := strings.TrimLeft(afterKey[1:], " \n\r\t")
		if afterColon == "" {
			return 0, false
		}
		if !strings.HasPrefix(afterColon, `"`) {
			searchFrom = index + len(needle)
			continue
		}
		return len(data) - len(afterColon) + 1, true
	}
}
