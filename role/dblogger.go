package role

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm/logger"
)

type GormLogger struct {
	Log zerolog.Logger
}

func (g GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return g
}

func (g GormLogger) Info(ctx context.Context, s string, i ...interface{}) {
	g.Log.Info().Msgf(s, i...)
}

func (g GormLogger) Warn(ctx context.Context, s string, i ...interface{}) {
	g.Log.Warn().Msgf(s, i...)
}

func (g GormLogger) Error(ctx context.Context, s string, i ...interface{}) {
	g.Log.Error().Msgf(s, i...)
}

func (g GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	sql, rowsAffected := fc()
	if err != nil {
		g.Log.Error().Time("begin", begin).Dur("elapsed", time.Since(begin)).Int64("rowsAffected", rowsAffected).Err(err).Msg(sql)
	} else {
		g.Log.Trace().Time("begin", begin).Dur("elapsed", time.Since(begin)).Int64("rowsAffected", rowsAffected).Msg(sql)
	}
}
