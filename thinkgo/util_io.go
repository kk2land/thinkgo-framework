package thinkgo

///*
//#include <unistd.h>
//int isatty(int fd);
//*/
//import "C"
import (
	"github.com/erikdubbelboer/gspt"
	"golang.org/x/term"
	"io"
	"os"
)

// IsAtty 判断是否在终端运行
var IsAtty = term.IsTerminal(int(os.Stdout.Fd())) //C.isatty(C.int(os.Stdout.Fd())) == 1

// WriteFile 写文件
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

// ReadFile 读文件
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

// IsDir 判断是否是目录
func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// IsFile 判断是否是文件
func IsFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

func SetProcessTitle(title string) {
	gspt.SetProcTitle(title)
}
