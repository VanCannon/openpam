package rdp

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// sendInstruction sends a Guacamole instruction to the writer
func (p *Proxy) sendInstruction(w io.Writer, opcode string, args ...string) error {
	var sb strings.Builder

	// Opcode
	sb.WriteString(fmt.Sprintf("%d.%s", len(opcode), opcode))

	// Args
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
	}

	sb.WriteString(";")

	_, err := w.Write([]byte(sb.String()))
	return err
}

// readInstruction reads a Guacamole instruction from the reader
func (p *Proxy) readInstruction(reader *bufio.Reader) (string, []string, error) {
	var elements []string
	var currentElement strings.Builder
	var length int

	for {
		// Read length
		lenStr, err := reader.ReadString('.')
		if err != nil {
			return "", nil, err
		}
		lenStr = strings.TrimSuffix(lenStr, ".")

		if _, err := fmt.Sscanf(lenStr, "%d", &length); err != nil {
			return "", nil, fmt.Errorf("invalid length: %w", err)
		}

		// Read content
		content := make([]byte, length)
		if _, err := io.ReadFull(reader, content); err != nil {
			return "", nil, err
		}
		currentElement.Write(content)
		elements = append(elements, currentElement.String())
		currentElement.Reset()

		// Check delimiter
		delim, err := reader.ReadByte()
		if err != nil {
			return "", nil, err
		}

		if delim == ';' {
			break
		} else if delim != ',' {
			return "", nil, fmt.Errorf("unexpected delimiter: %c", delim)
		}
	}

	if len(elements) == 0 {
		return "", nil, fmt.Errorf("empty instruction")
	}

	return elements[0], elements[1:], nil
}
