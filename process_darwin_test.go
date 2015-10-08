// +build darwin

package ps

import (
	"os"
	"testing"
)

func TestDarwinProcess_impl(t *testing.T) {
	var _ Process = new(DarwinProcess)
}

func TestGetArguments(t *testing.T) {
	args, err := GetArguments(os.Getpid())
	if err != nil {
		t.Fatalf("GetArguments() err: %v", err)
	}
	t.Logf("arguments: %v", args)
}

func TestGetAllProcessArguments(t *testing.T) {
	procs, err := Processes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(procs) <= 0 {
		t.Fatal("should have processes")
	}

	for _, p := range procs {
		args, err := p.Arguments()
		if err != nil {
			// seems to be a permissions issue
			t.Logf("no args for proc pid: %d ", p.Pid())
		} else {
			t.Logf("pid: %d, args: %v", p.Pid(), args)
		}
	}
}
