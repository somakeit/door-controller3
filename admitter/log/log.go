// log is an admitter that logs admissions using logrus
package log

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/somakeit/door-controller3/admitter"
)

type Log struct {
	logrus.Logger
}

func (l *Log) Interrogating(ctx context.Context, msg string) {
	l.WithFields(l.fields(ctx)).Info("Interrogating: ", msg)
}

func (l *Log) Deny(ctx context.Context, msg string, reason error) error {
	l.WithFields(l.fields(ctx)).Infof("Denied: %s, reason: %s", msg, reason)
	return nil
}

func (l *Log) Allow(ctx context.Context, msg string) error {
	l.WithFields(l.fields(ctx)).Info("Allowed: ", msg)
	return nil
}

func (l *Log) fields(ctx context.Context) logrus.Fields {
	return logrus.Fields{
		string(admitter.Door): ctx.Value(admitter.Door),
		string(admitter.Side): ctx.Value(admitter.Side),
		string(admitter.Type): ctx.Value(admitter.Type),
		string(admitter.ID):   ctx.Value(admitter.ID),
	}
}
