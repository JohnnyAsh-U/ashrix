package connector

import (
	"os"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	LoggerApp *zap.Logger
)

type LoggerConfig struct {
	Env string
	LogDir     string
	Level      string
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
}


func LoggerInit(cfg LoggerConfig) error {
	switch cfg.Env {
	case "dev":
		return initDev(cfg)
	case "prod":
		return initProd(cfg)
	default:
		return initProd(cfg)
	}
}



func initDev(cfg LoggerConfig) error {
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

	LoggerApp = zap.New(core, zap.AddCaller())

	return nil
}





func initProd(cfg LoggerConfig) error {
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

	LoggerApp = zap.New(core, zap.AddCaller(), zap.Fields(zap.String("service", "gateway")))

	return nil
}
