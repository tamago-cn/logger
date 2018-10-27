package logger

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/tamago-cn/cfg"

	"github.com/natefinch/lumberjack"
	"github.com/rifflock/lfshook"
	log "github.com/sirupsen/logrus"
)

func init() {
	levelMap = map[string]log.Level{
		"debug": log.DebugLevel,
		"info":  log.InfoLevel,
		"warn":  log.WarnLevel,
		"error": log.ErrorLevel,
		"DEBUG": log.DebugLevel,
		"INFO":  log.InfoLevel,
		"WARN":  log.WarnLevel,
		"ERROR": log.ErrorLevel,
	}
	conf = &LogConf{
		EnableConsole:   true,
		EnableTime:      true,
		EnablePos:       true,
		EnableColor:     true,
		TimestampFormat: "2006-01-02 15:04:05",
		LogFile:         "log/app.log",
		Level:           "info",
		MaxSize:         5,
		MaxDays:         30,
		MaxBackups:      5,
		Compress:        true,
	}
	cfg.RegistSection("log", conf, Reload, Destroy)
}

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 36
	gray    = 37
)

var (
	wg       sync.WaitGroup
	mCtx     context.Context
	mCancel  context.CancelFunc
	once     sync.Once
	conf     *LogConf
	levelMap map[string]log.Level
)

// Reload 重载日志
func Reload() error {
	Destroy()
	setLogger()
	//addLogger()
	if conf.MaxBackups > 1 {
		once.Do(addRotateLogger)
	} else {
		once.Do(addLogger)
	}
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		singalChangeLogLevel(ctx)
	}(ctx)
	mCancel = cancel
	return nil
}

func singalChangeLogLevel(ctx context.Context) {
	signalUser1 := make(chan os.Signal)
	signalUser2 := make(chan os.Signal)
	signal.Notify(signalUser1, syscall.SIGUSR1)
	signal.Notify(signalUser2, syscall.SIGUSR2)
	for {
		select {
		case <-ctx.Done():
			return
		case <-signalUser1:
			log.SetLevel(log.DebugLevel)
		case <-signalUser2:
			log.SetLevel(log.ErrorLevel)
		}
	}
}

// Destroy 析构
func Destroy() error {
	if mCancel != nil {
		mCancel()
	}
	wg.Wait()
	mCancel = nil
	return nil
}

// LogConf 日志配置
type LogConf struct {
	EnableConsole   bool   `ini:"enable_console" comment:"启用控制台打印"`
	EnableTime      bool   `ini:"enable_time" comment:"启用时间戳"`
	EnablePos       bool   `ini:"enable_pos" comment:"增加日志位置"`
	EnableColor     bool   `ini:"enable_color" json:"enable_color" comment:"日志颜色"`
	TimestampFormat string `ini:"timestamp_format" json:"timestamp_format" comment:"时间格式"`
	LogFile         string `ini:"log_file" json:"log_file" comment:"日志文件名"`
	Level           string `ini:"level" json:"level" comment:"日志等级"`
	MaxSize         int    `ini:"max_size" json:"max_size" comment:"日志文件大小最大值, 单位(MB)"`
	MaxDays         int    `ini:"max_days" json:"max_days" comment:"日志最大保存时间, 单位(天)"`
	MaxBackups      int    `ini:"mac_backups" json:"mac_backups" comment:"日志备份最大数量"`
	Compress        bool   `ini:"compress" json:"compress"  comment:"是否压缩"`
}

// LogFormatter 日志格式化
type LogFormatter struct {
	EnableTime      bool
	EnablePos       bool
	EnableColor     bool
	TimestampFormat string
	CallerLevel     int
}

// Format renders a single log entry
func (f *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	f.colored(b, entry, f.TimestampFormat)

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *LogFormatter) colored(b *bytes.Buffer, entry *log.Entry, timestampFormat string) {
	var levelColor int
	switch entry.Level {
	case log.DebugLevel:
		levelColor = gray
	case log.WarnLevel:
		levelColor = yellow
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	//// 封装层次较深
	//for i := 0; i < 20; i++ {
	//	_, file, line, ok := runtime.Caller(i)
	//	if !ok {
	//		file = "unknown"
	//		line = 0
	//	}
	//	fmt.Println(i, file, line)
	//}
	_, file, line, ok := runtime.Caller(f.CallerLevel)
	if !ok {
		file = "unknown"
		line = 0
	}
	file = path.Base(file)
	timePrefix := ""
	if f.EnableTime {
		timePrefix = fmt.Sprintf("%s ", entry.Time.Format(timestampFormat))
	}
	pos := ""
	if f.EnablePos {
		pos = fmt.Sprintf("[%s:%d] ", file, line)
	}
	levelText := strings.ToUpper(entry.Level.String())[0:4]
	if f.EnableColor {
		levelText = fmt.Sprintf("[\x1b[%dm%s\x1b[0m] ", levelColor, levelText)
	} else {
		levelText = fmt.Sprintf("[%s] ", levelText)
	}

	fmt.Fprintf(b, "%s%s%s%-44s ", timePrefix, pos, levelText, entry.Message)
}

// addLogger 内置命令，增加日志记录
func addLogger() {
	if level, ok := levelMap[conf.Level]; ok {
		log.SetLevel(level)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	lfHook := lfshook.NewHook(
		conf.LogFile,
		&LogFormatter{
			EnableTime:      conf.EnableTime,
			EnablePos:       conf.EnablePos,
			EnableColor:     conf.EnableColor,
			TimestampFormat: "2006-01-02 15:04:05",
			CallerLevel:     10,
		})
	log.AddHook(lfHook)
}

func addRotateLogger() {
	if level, ok := levelMap[conf.Level]; ok {
		log.SetLevel(level)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	lfHook := lfshook.NewHook(
		&lumberjack.Logger{
			Filename:   conf.LogFile,
			MaxSize:    conf.MaxSize,
			MaxAge:     conf.MaxDays,
			MaxBackups: conf.MaxBackups,
			LocalTime:  true,
			Compress:   conf.Compress,
		},
		&LogFormatter{
			EnableTime:      conf.EnableTime,
			EnablePos:       conf.EnablePos,
			EnableColor:     conf.EnableColor,
			TimestampFormat: "2006-01-02 15:04:05",
			CallerLevel:     10,
		})
	log.AddHook(lfHook)
}

// setLogger 设置默认日志格式
func setLogger() {
	if level, ok := levelMap[conf.Level]; ok {
		log.SetLevel(level)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if conf.EnableConsole {
		log.SetFormatter(&LogFormatter{
			EnableColor:     conf.EnableColor,
			TimestampFormat: "2006-01-02 15:04:05",
			CallerLevel:     7,
		})
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(&nullOutput{})
	}
}

type nullOutput struct {
}

func (w *nullOutput) Write(p []byte) (n int, err error) {
	return 0, nil
}
