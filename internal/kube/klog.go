package kube

import (
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

type KlogtoLogrus struct {
	Log logrus.FieldLogger
}

// Enabled tests whether this Logger is enabled.  For example, commandline
// flags might be used to set the logging verbosity and disable some info
// logs.
func (l *KlogtoLogrus) Enabled() bool {
	return true
}

// Info logs a non-error message with the given key/value pairs as context.
//
// The msg argument should be used to add some constant description to
// the log line.  The key/value pairs can then be used to add additional
// variable information.  The key/value pairs should alternate string
// keys and arbitrary values.
func (l *KlogtoLogrus) Info(msg string, keysAndValues ...interface{}) {
	l.Log.Debug(msg)
}

// Error logs an error, with the given message and key/value pairs as context.
// It functions similarly to calling Info with the "error" named value, but may
// have unique behavior, and should be preferred for logging errors (see the
// package documentations for more information).
//
// The msg field should be used to add context to any underlying error,
// while the err field should be used to attach the actual error that
// triggered this log line, if present.
func (l *KlogtoLogrus) Error(err error, msg string, keysAndValues ...interface{}) {
	l.Log.WithError(err).Debug(msg)
}

// V returns an Logger value for a specific verbosity level, relative to
// this Logger.  In other words, V values are additive.  V higher verbosity
// level means a log message is less important.  It's illegal to pass a log
// level less than zero.
func (l *KlogtoLogrus) V(level int) logr.Logger {
	return l
}

// WithValues adds some key-value pairs of context to a logger.
// See Info for documentation on how key/value pairs work.
func (l *KlogtoLogrus) WithValues(keysAndValues ...interface{}) logr.Logger {
	return l
}

// WithName adds a new element to the logger's name.
// Successive calls with WithName continue to append
// suffixes to the logger's name.  It's strongly recommended
// that name segments contain only letters, digits, and hyphens
// (see the package documentation for more information).
func (l *KlogtoLogrus) WithName(name string) logr.Logger {
	return l
}
