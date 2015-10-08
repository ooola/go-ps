// +build darwin

package ps

// #include "process_darwin.h"
import "C"

import (
	"bytes"
	"encoding/binary"
	"sync"
	"syscall"
	"unsafe"
)

// This lock is what verifies that C calling back into Go is only
// modifying data once at a time.
var darwinLock sync.Mutex
var darwinProcs []Process

type DarwinProcess struct {
	pid    int
	ppid   int
	binary string
}

func (p *DarwinProcess) Pid() int {
	return p.pid
}

func (p *DarwinProcess) PPid() int {
	return p.ppid
}

func (p *DarwinProcess) Executable() string {
	return p.binary
}

// this has to be dynamic because calling sysctl(...KERN_PROCARGS2...)
// fails on some processes with an "invalid argument" error
func (p *DarwinProcess) Arguments() ([]string, error) {

	args, err := GetArguments(p.pid)
	if err != nil {
		return nil, err
	}
	return args, nil
}

// copied from sys/sysctl.h
const (
	CTL_KERN       = 1
	KERN_PROC      = 14
	KERN_PROC_PID  = 1
	KERN_ARGMAX    = 8
	KERN_PROCARGS2 = 49
)

// returns the arguments passed to the specified process
// returns nil if no arguments are present
func GetArguments(pid int) ([]string, error) {
	rawBuf, length, err := getRawArguments(pid)

	if err != nil {
		return nil, err
	}

	// parse the arguments

	arguments := make([]string, 0)

	// read nargs
	i := 4
	var numArgs int32
	buf := bytes.NewBuffer(rawBuf[0:4])
	binary.Read(buf, binary.LittleEndian, &numArgs)

	for ; i < int(length); i++ {
		if rawBuf[i] == 0 {
			// skip exec_path
			break
		}
	}

	// skip any trailing NULLs
	for ; i < int(length); i++ {
		if rawBuf[i] != 0 {
			break
		}
	}
	if i == int(length) {
		// no arguments
		return nil, nil
	}

	argsStart := i
	argsFound := 0
	for ; i < int(length) && argsFound < int(numArgs); i++ {
		if rawBuf[i] == 0 {
			arguments = append(arguments, string(rawBuf[argsStart:i]))
			argsStart = i + 1
			argsFound++
		}
	}

	return arguments, nil
}

func getRawArguments(pid int) ([]byte, uint64, error) {
	// calculate maxArgSize
	maxArgSize := uint64(0)
	sizeOf := unsafe.Sizeof(maxArgSize)
	mib := []int32{CTL_KERN, KERN_ARGMAX}
	miblen := uint64(len(mib))
	_, _, err := syscall.RawSyscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(miblen),
		uintptr(unsafe.Pointer(&maxArgSize)),
		uintptr(unsafe.Pointer(&sizeOf)),
		0,
		0)
	if err != 0 {
		b := make([]byte, 0)
		return b, maxArgSize, err
	}

	// now we have the maxArgSize
	buffer := make([]byte, maxArgSize)

	// request KERN_PROCARGS2
	mib = []int32{CTL_KERN, KERN_PROCARGS2, int32(pid)}
	miblen = uint64(len(mib))
	_, _, err = syscall.RawSyscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(miblen),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&maxArgSize)),
		0,
		0)
	if err != 0 {
		// this seems to happen for some system processes
		// note that KERN_PROCARGS2 is wrapped in #if __APPLE_API_UNSTABLE in sysctl.h
		// fmt.Printf("KERN_PROCARGS2 failed for pid: %d\n", pid)
		b := make([]byte, 0)
		return b, maxArgSize, err
	}

	return buffer, maxArgSize, nil
}

//export go_darwin_append_proc
func go_darwin_append_proc(pid C.pid_t, ppid C.pid_t, comm *C.char) {
	proc := &DarwinProcess{
		pid:    int(pid),
		ppid:   int(ppid),
		binary: C.GoString(comm),
	}

	darwinProcs = append(darwinProcs, proc)
}

func findProcess(pid int) (Process, error) {
	ps, err := processes()
	if err != nil {
		return nil, err
	}

	for _, p := range ps {
		if p.Pid() == pid {
			return p, nil
		}
	}

	return nil, nil
}

func processes() ([]Process, error) {
	darwinLock.Lock()
	defer darwinLock.Unlock()
	darwinProcs = make([]Process, 0, 50)

	_, err := C.darwinProcesses()
	if err != nil {
		return nil, err
	}

	return darwinProcs, nil
}
