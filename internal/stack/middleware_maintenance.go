package stack

import (
	"net/http"
	"os"
	"strconv"

	"github.com/cybtachyon/sheidan/internal/maintenance"
	"github.com/gin-gonic/gin"
)

// defaultMaintenanceEnv is the standard engagement variable deploy
// environments already speak.
const defaultMaintenanceEnv = "SHEIDAN_MAINTENANCE"

// defaultMaintenanceFlagVar names the variable that points at the
// maintenance flag file, so deploy scripts toggle the gate without
// restarting the process.
const defaultMaintenanceFlagVar = "SHEIDAN_MAINTENANCE_FLAG"

// intakeBypass stamps a request that sails through a maintenance
// allowlist. The intake guards standing behind the gate (postsize and
// the header checks) stand down for stamped requests, so a monitor
// probing during maintenance never trips them.
const intakeBypass = "sheidan:intake.bypass"

// MaintenanceParams tunes the gate.maintenance slot. Zero selections
// recover the shipped defaults: engagement rides the standard
// environment variable and the standard flag-file variable, the retry
// hint waits sixty seconds, and the health probe and static families
// stay live. EnvVars names variables whose truthy value engages, and
// FlagVars names variables whose value is the path of a flag file.
// Both sides resolve per request, so flipping either takes effect on
// the next request without a restart.
type MaintenanceParams struct {
	EnvVars      []string
	FlagVars     []string
	RetrySeconds int
	LivePaths    []string
}

// acceptsMaintenanceParams reports whether the bag shapes the
// gate.maintenance slot expects.
func acceptsMaintenanceParams(params any) bool {
	_, ok := params.(*MaintenanceParams)
	return ok
}

// newMaintenance builds the gate.maintenance slot middleware. While
// engaged, the gate answers 503 with a Retry-After hint and a body
// that names the effective retry interval, so operators can tune
// alerting off it. Admitted paths carry the intake-bypass stamp into
// the rest of the chain.
func newMaintenance(params any) gin.HandlerFunc {
	cfg := MaintenanceParams{}
	if params != nil {
		cfg = *(params.(*MaintenanceParams))
	}
	envVars := cfg.EnvVars
	if len(envVars) == 0 {
		envVars = []string{defaultMaintenanceEnv}
	}
	flagVars := cfg.FlagVars
	if len(flagVars) == 0 {
		flagVars = []string{defaultMaintenanceFlagVar}
	}
	retry := cfg.RetrySeconds
	if retry <= 0 {
		retry = 60
	}
	var actives []maintenance.Activator
	for _, name := range envVars {
		actives = append(actives, maintenance.Env(name))
	}
	for _, varName := range flagVars {
		// The variable indirection resolves per request, so a
		// deploy can repoint the flag file without a rebuild.
		actives = append(actives, func() bool {
			path := os.Getenv(varName)
			if path == "" {
				return false
			}
			_, err := os.Stat(path)
			return err == nil
		})
	}
	engage := maintenance.Any(actives...)
	live := maintenance.DefaultLive
	if len(cfg.LivePaths) > 0 {
		live = nil
		for _, p := range cfg.LivePaths {
			live = append(live, maintenance.LiveRule(p))
		}
	}
	return func(c *gin.Context) {
		if !engage() {
			c.Next()
			return
		}
		if maintenance.Lives(live, c.Request.URL.Path) {
			c.Set(intakeBypass, true)
			c.Next()
			return
		}
		c.Header("Retry-After", strconv.Itoa(retry))
		c.AbortWithStatusJSON(http.StatusServiceUnavailable,
			gin.H{"error": "service is down for maintenance", "retry_seconds": retry})
	}
}
