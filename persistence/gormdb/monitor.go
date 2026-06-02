package gormdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	gormlogger "gorm.io/gorm/logger"
	gormutils "gorm.io/gorm/utils"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

const (
	logColorReset       = "\033[0m"
	logColorRed         = "\033[31m"
	logColorGreen       = "\033[32m"
	logColorYellow      = "\033[33m"
	logColorMagenta     = "\033[35m"
	logColorBlueBold    = "\033[34;1m"
	logColorMagentaBold = "\033[35;1m"
	logColorRedBold     = "\033[31;1m"
	logRows             = "[rows:%v]"
	logTime             = "[%.3fms] "
	slowQueryThreshold  = 300
	nanoToMs            = 1e6
)

type traceMsgConfig struct {
	trace     string
	traceWarn string
	traceErr  string
}

type wrappedMonitor struct {
	m              monitoring.Monitor
	shouldLogError func(error) bool
	slowThreshold  time.Duration
	traceMsgConfig traceMsgConfig
	logLevel       gormlogger.LogLevel
}

func (wm *wrappedMonitor) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *wm
	newLogger.logLevel = level

	return &newLogger
}

func (wm *wrappedMonitor) fmtLogMsg(msg string, data ...any) string {
	if len(data) > 0 {
		return fmt.Sprintf(msg+" extra data: %v", data)
	}

	return msg
}

func (wm *wrappedMonitor) Info(ctx context.Context, msg string, data ...any) {
	if wm.logLevel >= gormlogger.Info {
		wm.m.Logger().InfoContext(ctx, wm.fmtLogMsg(msg, data...))
	}
}

func (wm *wrappedMonitor) Warn(ctx context.Context, msg string, data ...any) {
	if wm.logLevel >= gormlogger.Warn {
		wm.m.Logger().WarnContext(ctx, wm.fmtLogMsg(msg, data...))
	}
}

func (wm *wrappedMonitor) Error(ctx context.Context, msg string, data ...any) {
	if wm.logLevel >= gormlogger.Error {
		wm.m.Logger().ErrorContext(ctx, wm.fmtLogMsg(msg, data...))
	}
}

func (wm *wrappedMonitor) fmtLogRows(rows int64) string {
	if rows == -1 {
		return "-"
	}

	return fmt.Sprintf("%d", rows)
}

func (wm *wrappedMonitor) mustWarnSlowQuery(spentTime time.Duration) bool {
	return spentTime > wm.slowThreshold
}

func (wm *wrappedMonitor) mustLogErr(err error) bool {
	if wm.shouldLogError == nil {
		return true
	}
	return wm.shouldLogError(err)
}

func (wm *wrappedMonitor) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if wm.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && wm.logLevel >= gormlogger.Error:
		sql, rows := fc()
		if wm.mustLogErr(err) {
			wm.m.Logger().ErrorContext(ctx,
				fmt.Sprintf(
					wm.traceMsgConfig.traceErr,
					gormutils.FileWithLineNum(), err.Error(),
					float64(elapsed.Nanoseconds())/nanoToMs, wm.fmtLogRows(rows), sql,
				),
			)
		} else {
			wm.m.Logger().InfoContext(ctx,
				fmt.Sprintf(
					wm.traceMsgConfig.trace,
					gormutils.FileWithLineNum(), float64(elapsed.Nanoseconds())/nanoToMs,
					wm.fmtLogRows(rows), sql,
				),
			)
		}
	case wm.logLevel >= gormlogger.Warn && wm.mustWarnSlowQuery(elapsed):
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", wm.slowThreshold)
		wm.m.Logger().WarnContext(ctx,
			fmt.Sprintf(
				wm.traceMsgConfig.traceWarn,
				gormutils.FileWithLineNum(), slowLog,
				float64(elapsed.Nanoseconds())/nanoToMs, wm.fmtLogRows(rows), sql,
			),
		)
	case wm.logLevel == gormlogger.Info:
		sql, rows := fc()
		wm.m.Logger().InfoContext(ctx,
			fmt.Sprintf(
				wm.traceMsgConfig.trace,
				gormutils.FileWithLineNum(), float64(elapsed.Nanoseconds())/nanoToMs,
				wm.fmtLogRows(rows), sql,
			),
		)
	}
}

type monitorConfig struct {
	shouldLogError     func(error) bool
	SlowQueryThreshold time.Duration
	logLevel           gormlogger.LogLevel
}

type MonitorOption func(c *monitorConfig)

func SlowQueriesThreshold(slowQueryThreshold time.Duration) MonitorOption {
	return func(c *monitorConfig) {
		c.SlowQueryThreshold = slowQueryThreshold
	}
}

func withLogLevelFromEnv() MonitorOption {
	return func(c *monitorConfig) {
		var logLevel gormlogger.LogLevel
		dbLogLevel := os.Getenv("DB_LOG_LEVEL")
		if len(dbLogLevel) < 1 {
			panic(errors.New("config.DB_LOG_LEVEL cannot be empty"))
		}
		dbLoggerLevel := logger.ParseLevel(dbLogLevel)
		switch dbLoggerLevel {
		case logger.LogLevelError, logger.LogLevelCritical:
			logLevel = gormlogger.Error
		case logger.LogLevelWarn:
			logLevel = gormlogger.Warn
		case logger.LogLevelInfo, logger.LogLevelDebug:
			logLevel = gormlogger.Info
		}

		c.logLevel = logLevel
	}
}

func defaultMonitorOptions() []MonitorOption {
	return []MonitorOption{
		SlowQueriesThreshold(slowQueryThreshold * time.Millisecond),
		withLogLevelFromEnv(),
	}
}

func newMonitor(m monitoring.Monitor, options ...MonitorOption) gormlogger.Interface {
	var (
		logTraceStr     = "%s\n[%.3fms] [rows:%v] %s"
		logTraceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		logTraceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if m.Logger().Enabled(int(logger.LogLevelDebug)) {
		logTraceStr = logColorGreen + "%s\n" + logColorReset + logColorYellow + logTime + logColorBlueBold + logRows + logColorReset + " %s"
		logTraceWarnStr = logColorGreen + "%s " + logColorYellow + "%s\n" + logColorReset + logColorRedBold + logTime +
			logColorYellow + logRows + logColorMagenta + " %s" + logColorReset
		logTraceErrStr = logColorRedBold + "%s " + logColorMagentaBold + "%s\n" + logColorReset + logColorYellow +
			logTime + logColorBlueBold + logRows + logColorReset + " %s"
	}

	c := &monitorConfig{shouldLogError: func(error) bool { return true }}
	for _, opt := range append(defaultMonitorOptions(), options...) {
		opt(c)
	}

	return &wrappedMonitor{
		m:              m,
		shouldLogError: c.shouldLogError,
		slowThreshold:  c.SlowQueryThreshold,
		traceMsgConfig: traceMsgConfig{
			trace:     logTraceStr,
			traceWarn: logTraceWarnStr,
			traceErr:  logTraceErrStr,
		},
		logLevel: c.logLevel,
	}
}
