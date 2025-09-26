/*
gura

Copyright (c) 2024-2025 Fyra Labs

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package util

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/mdobak/go-xerrors"
	"go.uber.org/zap"
)

// import "github.com/rs/zerolog"

var dotenvLoaded = false

// Initialise logger with given package prefix
func SetupLog(prefix string) *zap.Logger {
	if !dotenvLoaded {
		if err := godotenv.Load(); err != nil {
			fmt.Println("WARN(SetupLog):", err)
		}
		dotenvLoaded = true
	}
	if os.Getenv("GURA_DEV") == "1" {
		cfg := zap.NewDevelopmentConfig()
		// FIXME: this doesn't do anything?
		cfg.EncoderConfig.NameKey = prefix
		l := zap.Must(cfg.Build())
		return l
	}
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.NameKey = prefix
	l := zap.Must(cfg.Build())
	return l
}

// Suicide if `e` is not nil
func MaybeSuicide(l *zap.Logger, msg string, e error, fields ...zap.Field) {
	if e != nil {
		xerrors.Print(e)
		l.Fatal(msg, fields...)
	}
}

// Yeet if `e` is not nil
//
// Return whether an error was actually encountered
func Yeet(l *zap.Logger, msg string, e error, fields ...zap.Field) bool {
	if e != nil {
		xerrors.Print(e)
		l.Error(msg, fields...)
		return true
	}
	return false
}

func Assert(b bool, v ...any) {
	if len(v) == 0 {
		v = []any{"Assertion failed"}
	}
	if !b {
		panic(v[0])
	}
}
