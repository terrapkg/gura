package util

import (
	"os"

	"github.com/mdobak/go-xerrors"
	"go.uber.org/zap"
)

// import "github.com/rs/zerolog"

func SetupLog(prefix string) *zap.Logger {
	if os.Getenv("GURA_DEV") == "1" {
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.NameKey = prefix
		l := zap.Must(cfg.Build())
		return l
	}
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.NameKey = prefix
	l := zap.Must(cfg.Build())
	return l
}

func MaybeSuicide(l *zap.Logger, msg string, e error, fields ...zap.Field) {
	if e != nil {
		xerrors.Print(e)
		l.Fatal(msg, fields...)
	}
}

func Yeet(l *zap.Logger, msg string, e error, fields ...zap.Field) bool {
	if e != nil {
		xerrors.Print(e)
		l.Error(msg, fields...)
		return true
	}
	return false
}
