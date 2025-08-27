package logs

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

var logHnd zerolog.Logger

func init() {
	curProcID := strconv.Itoa(os.Getpid())

	zerolog.LevelFieldName = "L"
	zerolog.CallerFieldName = "F"
	zerolog.MessageFieldName = "Msg"
	zerolog.TimestampFieldName = "T"
	zerolog.FloatingPointPrecision = 2
	zerolog.TimeFieldFormat = "20060102-150405.000"
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		var b strings.Builder
		b.WriteString(curProcID + "/")
		b.WriteString(filepath.Base(file))

		// b.WriteString("/")
		// callerFunc := runtime.FuncForPC(pc).Name()
		// n := strings.Split(callerFunc, ".")
		// callerFunc = n[len(n)-1]
		// b.WriteString(callerFunc)

		b.WriteString(":" + strconv.Itoa(line))
		return b.String()
	}

	// 默认日志对象
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	consoleWriter := zerolog.ConsoleWriter{Out: zerolog.SyncWriter(os.Stdout), NoColor: true, TimeFormat: zerolog.TimeFieldFormat}
	logHnd = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
}

func LogsInit(logPath string, logLev, maxSize, maxBackups int) {
	zerolog.SetGlobalLevel(zerolog.Level(logLev)) //zerolog.DebugLevel)

	if len(logPath) < 1 {
		return
	}

	// logWriter := &lumberjack.Logger{
	// 	Filename:   logPath,
	// 	MaxSize:    maxSize >> 20, // MB
	// 	MaxBackups: maxBackups,
	// 	MaxAge:     0, // Days
	// 	LocalTime:  true,
	// 	//Compress:   true,
	// }
	log.Println("logPath:", logPath)
	logWriter := NewRollWriter(logPath, maxSize, maxBackups)
	consoleWriter := zerolog.ConsoleWriter{Out: logWriter, NoColor: true, TimeFormat: zerolog.TimeFieldFormat}
	logHnd = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()

	// } else {
	// 	consoleWriter := zerolog.ConsoleWriter{Out: zerolog.SyncWriter(os.Stdout), NoColor: true, TimeFormat: zerolog.TimeFieldFormat} // &lckPrint{Writer: os.Stdout}
	// 	logHnd = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
	// }
}

type zlogger struct {
	*zerolog.Event
}

func Trace() *zlogger {
	z := &zlogger{logHnd.Trace()}
	return z // .getpid()
}
func Debug() *zlogger {
	z := &zlogger{logHnd.Debug()}
	return z // .getpid()
}
func Info() *zlogger {
	z := &zlogger{logHnd.Info()}
	return z // .getpid()
}
func Warn(err error) *zlogger {
	z := &zlogger{logHnd.Warn()}
	return z.Fail(err) // .getpid()
}
func Error(err error) *zlogger {
	z := &zlogger{logHnd.Error()}
	return z.Fail(err) // .getpid()
}
func Fatal(err error) *zlogger {
	z := &zlogger{logHnd.Fatal()}
	return z.Fail(err) // .getpid()
}
func Panic(err error) *zlogger {
	z := &zlogger{logHnd.Panic()}
	return z.Fail(err) // .getpid()
}

func (z *zlogger) Fail(err error) *zlogger {
	if err != nil {
		z.AnErr("err", err)
	}

	return z
}

func (z *zlogger) Rid(rid string) *zlogger {
	z.Str("rid", rid)
	return z
}
func Catch(err error) *zlogger {
	const size = 16 << 10
	buf := make([]byte, size)
	buf = buf[:runtime.Stack(buf, false)]
	var sb strings.Builder
	sb.Write(buf)
	if err != nil {
		log.Printf("err: %v\n", err)
	}
	log.Printf("stack: %s\n", sb.String())

	z := &zlogger{logHnd.Fatal()}
	z.Str("stack", sb.String())
	return z
}

type ExternWriter struct{}

var ExtWriter = ExternWriter{}

func (ew ExternWriter) Write(p []byte) (n int, err error) {
	logHnd.Debug().Bytes("ew", p).Send()
	return len(p), nil
}
