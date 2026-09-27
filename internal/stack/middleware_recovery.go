package stack

import (
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"

	gsrequestid "github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// guardedRecovery returns the recovery middleware installed ahead of
// the chain. Unlike gin's stock recovery, it records the panic
// through the logging slot's writer, so a crashed exchange leaves the
// same kind of structured trail as a graded 5xx response. The
// request identifier rides along through the request.id slot's
// request-header stamp, which survives the unwind.
func guardedRecovery(logWriter io.Writer) gin.HandlerFunc {
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelError}))
	return func(c *gin.Context) {
		defer func() {
			err := recover()
			if err == nil {
				return
			}
			fields := []any{"panic", fmt.Sprintf("%v", err)}
			if id := gsrequestid.Get(c); id != "" {
				fields = append(fields, "request_id", id)
			}
			logger.Error("recovered from panic", fields...)
			logger.Error("panic stack", "stack", string(debug.Stack()))
			c.AbortWithStatus(500)
		}()
		c.Next()
	}
}

// slogWriter resolves the writer the logging.slog slot feeds, falling
// back to os.stdout so the recovery trail shares the console narrative
// with the ordinary records.
func (b *Builder) slogWriter() io.Writer {
	for _, e := range b.entries {
		if e.spec == nil || e.spec.Name != LoggingSlog {
			continue
		}
		if params, ok := e.params.(*SlogParams); ok && params.Writer != nil {
			return params.Writer
		}
	}
	return defaultLogWriter
}

// Assemble builds the engine for this builder: the guarded recovery
// ahead of the compiled chain, trusted proxy parsing disabled. It is
// the single assembly point for sheidan.New and the Stack builder.
func (b *Builder) Assemble() (*gin.Engine, error) {
	handlers, err := b.Compile()
	if err != nil {
		return nil, err
	}
	engine := gin.New()
	// A nil list returns without an error, so the discard is safe.
	_ = engine.SetTrustedProxies(nil)
	all := append(make([]gin.HandlerFunc, 0, len(handlers)+1), guardedRecovery(b.slogWriter()))
	all = append(all, handlers...)
	engine.Use(all...)
	return engine, nil
}
