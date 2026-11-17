package models

import "time"

type DemoSettings struct {
	Enabled             bool            `bson:"enabled" json:"enabled"`
	Photo               bool            `bson:"photo" json:"photo"`
	Strip               bool            `bson:"strip" json:"strip"`
	Boomerang           bool            `bson:"boomerang" json:"boomerang"`
	StickerAssetIDs     []string        `bson:"stickerAssetIds,omitempty" json:"stickerAssetIds,omitempty"`
	Countdowns          EventCountdowns `bson:"countdowns" json:"countdowns"`
	CountdownsSet       bool            `bson:"countdownsSet,omitempty" json:"-"`
	PhotoTemplateID     string          `bson:"photoTemplateId,omitempty" json:"photoTemplateId,omitempty"`
	StripTemplateID     string          `bson:"stripTemplateId,omitempty" json:"stripTemplateId,omitempty"`
	BoomerangTemplateID string          `bson:"boomerangTemplateId,omitempty" json:"boomerangTemplateId,omitempty"`
}

type PlatformSettings struct {
	Key       string       `bson:"key"`
	Demo      DemoSettings `bson:"demo" json:"demo"`
	UpdatedAt time.Time    `bson:"updatedAt" json:"updatedAt"`
}
