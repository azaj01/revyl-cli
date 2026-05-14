package hotreload

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestIsLocalConnectionRefused(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "syscall refused",
			err:  syscall.ECONNREFUSED,
			want: true,
		},
		{
			name: "wrapped net op refused",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
			},
			want: true,
		},
		{
			name: "string wrapped websocket refused",
			err:  fmt.Errorf("failed to connect local websocket: dial tcp :8081: connect: connection refused"),
			want: true,
		},
		{
			name: "windows actively refused",
			err:  fmt.Errorf("dial tcp 127.0.0.1:8081: connectex: No connection could be made because the target machine actively refused it."),
			want: true,
		},
		{
			name: "non refused network error",
			err:  fmt.Errorf("dial tcp 127.0.0.1:8081: i/o timeout"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalConnectionRefused(tt.err); got != tt.want {
				t.Fatalf("isLocalConnectionRefused(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
