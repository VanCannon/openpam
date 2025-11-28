package rdp

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestReadInstruction(t *testing.T) {
	proxy := &Proxy{}

	tests := []struct {
		name           string
		input          string
		expectedOpcode string
		expectedArgs   []string
	}{
		{
			name:           "Standard args",
			input:          "4.args,13.VERSION_1_5_0,8.hostname,4.port,8.username,8.password,6.domain,8.security,11.ignore-cert;",
			expectedOpcode: "args",
			expectedArgs:   []string{"VERSION_1_5_0", "hostname", "port", "username", "password", "domain", "security", "ignore-cert"},
		},
		{
			name:           "Empty args",
			input:          "4.args,13.VERSION_1_5_0;",
			expectedOpcode: "args",
			expectedArgs:   []string{"VERSION_1_5_0"},
		},
		{
			name:           "Args with values containing commas",
			input:          "4.args,13.VERSION_1_5_0,3.foo,3.b,r,3.baz;",
			expectedOpcode: "args",
			expectedArgs:   []string{"VERSION_1_5_0", "foo", "b,r", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			opcode, args, err := proxy.readInstruction(reader)
			if err != nil {
				t.Fatalf("readInstruction() error = %v", err)
			}
			if opcode != tt.expectedOpcode {
				t.Errorf("opcode = %v, want %v", opcode, tt.expectedOpcode)
			}
			if !reflect.DeepEqual(args, tt.expectedArgs) {
				t.Errorf("args = %v, want %v", args, tt.expectedArgs)
			}
		})
	}
}
