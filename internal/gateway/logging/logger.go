package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

var (
	App *zap.Logger
	Audit *auditLogger
)

type Config struct {
	Env string
	LogDir     string
	Level      string
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
}


func Init(cfg Config) error {
	switch cfg.Env {
	case "dev":
		return initDev(cfg)
	case "prod":
		return initProd(cfg)
	default:
		return initProd(cfg)
	}
}



func initDev(cfg Config) error {
	level := zapcore.DebugLevel

	consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey: "T",
		LevelKey: "L",
		NameKey: "N",
		CallerKey: "C",
		MessageKey: "M",
		LineEnding: zapcore.DefaultLineEnding,
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
		EncodeTime: zapcore.TimeEncoderOfLayout("15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	})

	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	App = zap.New(core, zap.AddCaller())

	// Audit log
	auditFile, err := os.OpenFile(
		cfg.LogDir + "/audit.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0640,
	)

	if err != nil {
		return err
	}

	Audit = newAuditLogger(auditFile)
	return nil
}





func initProd(cfg Config) error {
	if err := os.MkdirAll(cfg.LogDir, 0750); err != nil {
		return err
	}
	level := zapcore.InfoLevel
	_ = level.UnmarshalText([]byte(cfg.Level))

	appWriter := &lumberjack.Logger{
		Filename:   cfg.LogDir + "/gateway.log",
		MaxSize:    cfg.MaxSizeMB,
		MaxAge:     cfg.MaxAgeDays,
		MaxBackups: cfg.MaxBackups,
		Compress:   cfg.Compress,
	}

	jsonEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		MessageKey:     "msg",
		CallerKey:      "caller",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseColorLevelEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})

	multiWriter := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(appWriter),
	)

	core := zapcore.NewCore(jsonEncoder, multiWriter, level)

	App = zap.New(core, zap.AddCaller(), zap.Fields(zap.String("service", "gateway")))

	// Audit log
	auditFile, err := os.OpenFile(
		cfg.LogDir + "/audit.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0640,
	)

	if err != nil {
		return err
	}

	Audit = newAuditLogger(auditFile)

	return nil

}
