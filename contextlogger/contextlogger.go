package contextlogger

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/somakeit/door-controller3/admitter"
)

// ContextLogger is an adapter to logrus for the log calls in this module. It
// also directly impliments the admitter interface.
type ContextLogger struct {
	Logger *logrus.Logger
}

func (c *ContextLogger) Fatal(ctx context.Context, args ...interface{}) {
	c.Logger.WithFields(c.fields(ctx)).Fatal(args...)
}

func (c *ContextLogger) Debug(ctx context.Context, args ...interface{}) {
	c.Logger.WithFields(c.fields(ctx)).Debug(args...)
}

func (c *ContextLogger) Interrogating(ctx context.Context, msg string) {
	c.Logger.WithFields(c.fields(ctx)).Info("Interrogating: ", msg)
}

func (c *ContextLogger) Deny(ctx context.Context, msg string, reason error) error {
	c.Logger.WithFields(c.fields(ctx)).Infof("Denied: %s, reason: %s", msg, reason)
	return nil
}

func (c *ContextLogger) Allow(ctx context.Context, msg string) error {
	c.Logger.WithFields(c.fields(ctx)).Info("Allowed: ", msg)
	return nil
}

func (c *ContextLogger) fields(ctx context.Context) logrus.Fields {
	return logrus.Fields{
		string(admitter.Door): ctx.Value(admitter.Door),
		string(admitter.Side): ctx.Value(admitter.Side),
		string(admitter.Type): ctx.Value(admitter.Type),
		string(admitter.ID):   ctx.Value(admitter.ID),
	}
}
