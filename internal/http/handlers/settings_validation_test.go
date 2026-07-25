package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/mayvqt/veyra/internal/integrations/seerr"
	"github.com/mayvqt/veyra/internal/store"
)

func TestValidateSettingsInputValid(t *testing.T) {
	in := map[string]string{
		settingAppName:              "Veyra",
		settingAppLogoURL:           "https://example.com/logo.svg",
		settingAppAccentColor:       "#d43f24",
		settingMediaServerPublicURL: "https://mediaserver.example.com",
		settingSeerrPublicURL:       "https://seerr.example.com",
	}
	if msg := validateSettingsInput(in); msg != "" {
		t.Fatalf("expected valid input, got %s", msg)
	}
}

func TestValidateSettingsInputRejectsBadColor(t *testing.T) {
	in := map[string]string{
		settingAppName:              "Veyra",
		settingAppLogoURL:           "",
		settingAppAccentColor:       "red",
		settingMediaServerPublicURL: "https://mediaserver.example.com",
		settingSeerrPublicURL:       "https://seerr.example.com",
	}
	if msg := validateSettingsInput(in); msg == "" {
		t.Fatal("expected validation error for color")
	}
}

func TestValidateSettingsInputRejectsBadURLsAndName(t *testing.T) {
	in := map[string]string{
		settingAppName:              "A",
		settingAppLogoURL:           "ftp://invalid.example.com/logo.svg",
		settingAppAccentColor:       "#d43f24",
		settingMediaServerPublicURL: "notaurl",
		settingSeerrPublicURL:       "https://seerr.example.com",
	}
	if msg := validateSettingsInput(in); msg == "" {
		t.Fatal("expected validation error")
	}
}

func TestReadBoolSettingParsesFlexibleValues(t *testing.T) {
	db := mkDB(t)
	defer db.Close()
	r := httptest.NewRequest("GET", "/", nil)

	if err := store.UpsertSetting(context.Background(), db, "widget.flag", "1"); err != nil {
		t.Fatal(err)
	}
	if !readBoolSetting(r, db, "widget.flag", false) {
		t.Fatal("expected numeric true to parse")
	}
	if err := store.UpsertSetting(context.Background(), db, "widget.flag", "FALSE"); err != nil {
		t.Fatal(err)
	}
	if readBoolSetting(r, db, "widget.flag", true) {
		t.Fatal("expected uppercase false to parse")
	}
	if err := store.UpsertSetting(context.Background(), db, "widget.flag", "invalid"); err != nil {
		t.Fatal(err)
	}
	if !readBoolSetting(r, db, "widget.flag", true) {
		t.Fatal("expected fallback on invalid bool")
	}
}

func TestQuotaViewValuesIncludeMeterData(t *testing.T) {
	display := quotaViewValues(seerr.Quota{
		MovieLimit:      5,
		MovieUsed:       3,
		MovieRemaining:  2,
		SeriesLimit:     4,
		SeriesUsed:      4,
		SeriesRemaining: 0,
	})

	if len(display.Meters) != 2 {
		t.Fatalf("expected two quota meters, got %+v", display.Meters)
	}
	movie := display.Meters[0]
	if movie.Value != "2/5 left" || movie.Class != "warn" || movie.Detail != "3 used" || movie.Percent != 40 {
		t.Fatalf("unexpected movie quota display: %+v", movie)
	}
	series := display.Meters[1]
	if series.Value != "0/4 left" || series.Class != "bad" || series.Detail != "4 used" || series.Percent != 0 {
		t.Fatalf("unexpected series quota display: %+v", series)
	}
}

func TestQuotaViewValuesUnlimitedMeterIsFull(t *testing.T) {
	display := quotaViewValues(seerr.Quota{
		MovieUnlimited:  true,
		SeriesUnlimited: true,
	})

	if len(display.Meters) != 2 {
		t.Fatalf("expected two quota meters, got %+v", display.Meters)
	}
	for _, meter := range display.Meters {
		if meter.Value != "Unlimited" || meter.Class != "ok" || meter.Percent != 100 || meter.Detail != "No limit" {
			t.Fatalf("unexpected unlimited quota meter state: %+v", meter)
		}
	}
}
