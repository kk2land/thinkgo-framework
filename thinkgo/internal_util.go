package thinkgo

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// 内部功能函数

func checkProcessByPid(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err = proc.Signal(syscall.Signal(0)); err != nil {
		if err.Error() == "operation not permitted" {
			//无权限kill
			return true
		}
		return false
	}
	return true
}

func signalByPidFile(pidfile string, s os.Signal) error {
	var buf []byte
	var pid int
	var err error
	if buf, err = ReadFile(pidfile); err != nil {
		return fmt.Errorf("pidfile(%s)读取失败 - %s", pidfile, err)
	}
	if pid, err = strconv.Atoi(string(buf)); err != nil {
		return fmt.Errorf("pidfile内容不合法 - %s", string(buf))
	}
	return signalByPid(pid, s)
}

func signalByPid(pid int, s os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("os.FindProcess(%d)失败 - %s", pid, err)
	}
	if err = proc.Signal(s); err != nil {
		return fmt.Errorf("Process.Signal(%d)失败 - %s", s, err)
	}
	return nil
}
