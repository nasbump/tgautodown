package logs

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// osStat          = os.Stat
	osRename        = os.Rename
	osRemove        = os.Remove
	defaultFileMode = os.FileMode(0600)
	// defaultFileFlag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC // O_APPEND // os.O_TRUNC
	defaultFileFlag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	ErrOpenFileFail = errors.New("open file fail")
)

const zipSuffix = ".gz"

type RollWriter struct {
	Filename   string `json:"filename" yaml:"filename"`
	MaxSize    int64  `json:"maxsize" yaml:"maxsize"`
	MaxBackups int    `json:"maxbackups" yaml:"maxbackups"`
	Compress   bool   `json:"compress" yaml:"compress"`

	size   int64
	file   *os.File
	mu     sync.Mutex
	cleanC chan bool
	logDir string
}

func NewRollWriter(logPath string, maxSize, maxBackups int) *RollWriter {
	logWriter := &RollWriter{
		Filename:   logPath,
		MaxSize:    int64(maxSize),
		MaxBackups: maxBackups,
		Compress:   true,
		cleanC:     make(chan bool),
		logDir:     filepath.Dir(logPath),
	}

	if logWriter.MaxBackups > 1 { // need rotate
		go logWriter.cleanLogs()
	}
	return logWriter
}

func (l *RollWriter) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		if err = l.open(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)
	// log.Printf("write %d/%d, l.size=%d/%d, err=%+v", n, len(p), l.size, l.MaxSize, err)

	if l.size > l.MaxSize {
		l.rotate()
	}

	return n, err
}

func (l *RollWriter) open() error {
	f, err := os.OpenFile(l.Filename, defaultFileFlag, defaultFileMode)
	if err != nil {
		// return ErrOpenFileFail
		return fmt.Errorf("open logpath:%s fail: %s", l.Filename, err)
	}

	if info, err := f.Stat(); err == nil {
		l.size = info.Size()
	}

	l.file = f
	return nil
}

func (l *RollWriter) rotate() error {
	l.file.Sync()
	l.file.Close()
	l.file = nil
	l.size = 0

	if l.MaxBackups < 2 { // do not rotate
		osRemove(l.Filename)
		return nil
	}

	bakName := l.Filename + "-" + time.Now().Format(time.RFC3339) + ".log"
	if err := osRename(l.Filename, bakName); err != nil {
		return err
	}

	if l.Compress {
		go l.zipLog(bakName)
	} else {
		l.cleanC <- true
	}
	return nil
}

func (l *RollWriter) zipLog(srcName string) error {
	dstName := l.Filename + "-" + time.Now().Format(time.RFC3339) + zipSuffix

	f, err := os.Open(srcName)
	if err != nil {
		log.Println("zipLog", srcName, "open src fail:", err)
		return err
	}
	defer f.Close()

	gzf, err := os.Create(dstName)
	if err != nil {
		log.Println("zipLog", dstName, "create dst fail:", err)
		return err
	}
	defer gzf.Close()

	defer func() {
		if err != nil { // failed to compress log file
			osRemove(dstName)
		}
	}()

	gz := gzip.NewWriter(gzf)
	_, err = io.Copy(gz, f)
	if err != nil {
		log.Println("zipLog", "io.copy fail:", err)
		return err
	}

	err = gz.Close()
	if err != nil {
		log.Println("zipLog", "gz.close fail:", err)
		return err
	}

	if err := osRemove(srcName); err != nil { // remove oldfile if gzip succ
		log.Println("zipLog", srcName, "remove src fail:", err)
		return err
	}

	l.cleanC <- true
	return nil
}

func (l *RollWriter) cleanLogs() {
	for {
		<-l.cleanC
		l.cleanOldLogs()
	}
}

type FileInfo struct {
	Name    string
	ModTime time.Time
	Size    int64
}

func (l *RollWriter) cleanOldLogs() {
	// log.Printf("will scan dir:%s\n", l.logDir)

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		// log.Printf("will scan dir:%s, err=%v\n", l.logDir, err)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.IsDir() { // 忽略子目录
			continue
		}

		info, err := entry.Info()
		if err != nil {
			// log.Printf("will entry.Info:%s, err=%v\n", l.logDir, err)
			continue
		}
		name := info.Name()
		if strings.HasSuffix(name, zipSuffix) {
			// log.Printf("will scan name:%s ok\n", name)
			files = append(files, FileInfo{
				Name:    name,
				ModTime: info.ModTime(),
				Size:    info.Size(),
			})
		} else {
			// log.Printf("will scan name:%s skip\n", name)
		}
	}

	fcnt := len(files)
	// log.Printf("maxBackups:%d, fcnt:%d", l.MaxBackups, fcnt)
	if fcnt <= l.MaxBackups {
		return
	}

	// 3. 按修改时间排序（从新到旧）
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	for i := l.MaxBackups; i < fcnt; i++ {
		// log.Printf("will clean files[%d/%d].Name:%s", i, fcnt, files[i].Name)
		if err := osRemove(filepath.Join(l.logDir, files[i].Name)); err != nil {
			// log.Println("remove file fail:", err)
		}
	}

	// return files, nil
}

// func (l *RollWriter) rotate() error {
// 	l.file.Sync()
// 	l.file.Close()
// 	l.file = nil
// 	l.size = 0

// 	m := l.MaxBackups
// 	if m < 1 {
// 		m = 1
// 	}

// 	for i := m - 2; i > 0; i-- {
// 		curName := l.Filename + "." + strconv.Itoa(i)
// 		_, err := osStat(curName)
// 		if os.IsNotExist(err) {
// 			continue
// 		}

// 		bakName := l.Filename + "." + strconv.Itoa(i+1)
// 		osRename(curName, bakName)
// 	}

// 	if m > 1 {
// 		bakName := l.Filename + ".1" // + strconv.Itoa(1)
// 		osRename(l.Filename, bakName)
// 	} else {
// 		osRemove(l.Filename)
// 	}

// 	return nil
// }
