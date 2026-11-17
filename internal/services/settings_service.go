package services

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
)

const platformSettingsKey = "platform"

type SettingsService struct {
	repo     repositories.PlatformSettingsRepository
	defaults models.DemoSettings
}

func NewSettingsService(repo repositories.PlatformSettingsRepository, cfg config.DemoConfig) *SettingsService {
	return &SettingsService{
		repo: repo,
		defaults: models.DemoSettings{
			Enabled:       cfg.Enabled,
			Photo:         cfg.Photo,
			Strip:         cfg.Strip,
			Boomerang:     cfg.Boomerang,
			Countdowns:    models.EventCountdowns{Photo: cfg.Countdown, Strip: cfg.Countdown, Boomerang: cfg.Countdown}.Normalized(),
			CountdownsSet: true,
		},
	}
}

func (s *SettingsService) Demo(ctx context.Context) (models.DemoSettings, error) {
	settings, found, err := s.repo.Find(ctx, platformSettingsKey)
	if err != nil {
		return models.DemoSettings{}, err
	}
	if !found {
		return s.defaults, s.repo.Upsert(ctx, models.PlatformSettings{Key: platformSettingsKey, Demo: s.defaults})
	}
	if !settings.Demo.CountdownsSet {
		settings.Demo.Countdowns = s.defaults.Countdowns
		settings.Demo.CountdownsSet = true
		if err := s.repo.Upsert(ctx, models.PlatformSettings{Key: platformSettingsKey, Demo: settings.Demo}); err != nil {
			return models.DemoSettings{}, err
		}
	}
	return settings.Demo, nil
}

func (s *SettingsService) UpdateDemo(ctx context.Context, demo models.DemoSettings) (models.DemoSettings, error) {
	demo.Countdowns = demo.Countdowns.Normalized()
	demo.CountdownsSet = true
	settings := models.PlatformSettings{Key: platformSettingsKey, Demo: demo}
	if err := s.repo.Upsert(ctx, settings); err != nil {
		return models.DemoSettings{}, err
	}
	return demo, nil
}
