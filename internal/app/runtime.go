package app

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/thgrace/training-wheels/internal/config"
	"github.com/thgrace/training-wheels/internal/eval"
	"github.com/thgrace/training-wheels/internal/logger"
	"github.com/thgrace/training-wheels/internal/override"
	"github.com/thgrace/training-wheels/internal/packs"
	"github.com/thgrace/training-wheels/internal/packs/allpacks"
	"github.com/thgrace/training-wheels/internal/rules"
	"github.com/thgrace/training-wheels/internal/session"
	"github.com/thgrace/training-wheels/internal/shellcontext"
)

type packDirLoader interface {
	LoadFromDir(path string) error
}

type packFileLoader interface {
	LoadFile(path string) error
}

// EvalOptions controls optional evaluator setup beyond config and pack loading.
type EvalOptions struct {
	Shell         shellcontext.Shell
	Trace         eval.TraceCollector
	LoadOverrides bool
	LoadRules     bool
	LoadSession   bool
}

// LoadConfig loads the user/project configuration for the current invocation.
func LoadConfig() (*config.Config, error) {
	return config.Load()
}

// EnsureDefaultRegistry loads built-in and configured external packs into the default registry.
func EnsureDefaultRegistry() {
	if packs.DefaultRegistry().Count() > 0 {
		return
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Warn("external pack path resolution skipped", "error", err)
		return
	}

	InitializeRegistry(packs.DefaultRegistry(), cfg)
}

// InitializeRegistry registers built-in packs and configured external pack sources.
func InitializeRegistry(reg *packs.PackRegistry, cfg *config.Config) {
	allpacks.RegisterAll(reg)
	LoadPackSourcesFromConfig(reg, cfg)
}

// LoadPackSourcesFromConfig loads configured external pack sources into a registry.
func LoadPackSourcesFromConfig(reg *packs.PackRegistry, cfg *config.Config) {
	paths, err := cfg.ExternalPackPaths()
	if err != nil {
		logger.Warn("external pack path resolution skipped", "error", err)
		return
	}

	dirLoader, hasDirLoader := any(reg).(packDirLoader)
	fileLoader, hasFileLoader := any(reg).(packFileLoader)
	if !hasDirLoader && !hasFileLoader {
		return
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			logger.Warn("external pack source skipped", "path", path, "error", err)
			continue
		}

		if info.IsDir() {
			if hasDirLoader {
				if err := dirLoader.LoadFromDir(path); err != nil {
					logger.Warn("external pack directory load completed with errors", "path", path, "error", err)
				}
			}
			continue
		}

		if hasFileLoader {
			if err := fileLoader.LoadFile(path); err != nil {
				logger.Warn("external pack file load failed", "path", path, "error", err)
			}
		}
	}
}

// NewEvaluator builds a configured evaluator using the default pack registry.
func NewEvaluator(cfg *config.Config, opts EvalOptions) *eval.Evaluator {
	EnsureDefaultRegistry()

	evaluator := eval.NewEvaluator(cfg, packs.DefaultRegistry())
	if opts.Shell != nil {
		evaluator.SetShell(opts.Shell)
	}
	if opts.Trace != nil {
		evaluator.SetTrace(opts.Trace)
	}
	if opts.LoadOverrides {
		loadOverrides(evaluator)
	}
	if opts.LoadRules {
		loadRules(evaluator)
	}
	if opts.LoadSession {
		loadSessionAllowlist(evaluator)
	}

	return evaluator
}

// EvalContext returns a context with the configured hook timeout.
func EvalContext(cfg *config.Config) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.General.HookTimeoutMs) * time.Millisecond
	return context.WithTimeout(context.Background(), timeout)
}

func loadOverrides(evaluator *eval.Evaluator) {
	user, project, err := override.LoadMerged()
	if err != nil {
		logger.Warn("override load error, continuing without overrides", "error", err)
		return
	}
	evaluator.SetOverrides(user, project)
}

func loadRules(evaluator *eval.Evaluator) {
	userRulesPath, _ := rules.UserRulesPath()
	if userRulesPath == "" {
		return
	}

	userRules, rulesErr := rules.LoadOrCreate(userRulesPath)
	projectRules, projRulesErr := rules.LoadOrCreate(rules.ProjectRulesPath())
	if rulesErr != nil {
		logger.Warn("user rules load error, continuing without rules", "error", rulesErr)
	}
	if projRulesErr != nil {
		logger.Warn("project rules load error, continuing without rules", "error", projRulesErr)
	}
	if rulesErr == nil || projRulesErr == nil {
		evaluator.SetRules(userRules, projectRules)
	}
}

func loadSessionAllowlist(evaluator *eval.Evaluator) {
	token, _ := session.ReadToken()
	if token == "" {
		return
	}

	secret, err := session.LoadOrCreateSecret(session.SecretPath())
	if err != nil {
		return
	}

	allowlist, err := session.Load(token, secret)
	if err != nil {
		return
	}

	evaluator.SetSessionAllows(allowlist)
}
