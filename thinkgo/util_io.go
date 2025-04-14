package thinkgo

/*
#include <unistd.h>
int isatty(int fd);
*/
import "C"
import (
	"io"
	"os"
)

var IsAtty = C.isatty(C.int(os.Stdout.Fd())) == 1

func WriteFile(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func ReadFile(path string) ([]byte, error) {
	var err error
	var stat os.FileInfo
	var fp *os.File

	stat, err = os.Stat(path)
	if err != nil {
		return nil, err
	}

	fp, err = os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = fp.Close()
	}()

	var buf = make([]byte, stat.Size())
	_, err = fp.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func IsFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}
