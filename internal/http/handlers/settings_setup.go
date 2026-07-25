package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/mayvqt/veyra/internal/config"
	"github.com/mayvqt/veyra/internal/security"
)

func (h *Handlers) setupInputFromSettingsForm(r *http.Request) (config.SetupInput, bool, error) {
	stored, storedKeys, err := config.StoredSetup(r.Context(), h.db, security.NewCrypto(h.cfg.EncryptionKey))
	if err != nil {
		return config.SetupInput{}, false, err
	}
	if len(storedKeys) == 0 {
		return stored, false, nil
	}
	in := stored
	changed := false
	for _, field := range setupConfigFields {
		if field.envManaged() {
			continue
		}
		formValue := strings.TrimSpace(r.FormValue(field.name))
		if field.secret && formValue == "" {
			continue
		}
		if formValue != field.get(in) {
			field.set(&in, formValue)
			changed = true
		}
	}
	return in, changed, nil
}

func (h *Handlers) populateSetupConfigView(r *http.Request, view *ViewData, submitted config.SetupInput) {
	stored, storedKeys, err := config.StoredSetup(r.Context(), h.db, security.NewCrypto(h.cfg.EncryptionKey))
	if err != nil || len(storedKeys) == 0 {
		return
	}
	if submitted != (config.SetupInput{}) {
		stored = submitted
	}
	current := setupInputFromConfig(h.cfg)
	for _, field := range setupConfigFields {
		envManaged := field.envManaged()
		value := field.get(stored)
		if envManaged {
			value = field.get(current)
		}
		view.SetupConfigFields = append(view.SetupConfigFields, SetupConfigField{
			Section:    field.section,
			Label:      field.label,
			Name:       field.name,
			Value:      visibleSetupFieldValue(value, field.secret),
			Secret:     field.secret,
			EnvManaged: envManaged,
			HasValue:   value != "" || envManaged,
			Options:    field.optionViews(value),
		})
	}
	view.SetupConfigGroups = groupSetupConfigFields(view.SetupConfigFields)
}

func visibleSetupFieldValue(value string, secret bool) string {
	if secret {
		return ""
	}
	return value
}

func groupSetupConfigFields(fields []SetupConfigField) []SetupConfigGroup {
	groups := make([]SetupConfigGroup, 0, len(fields))
	groupIndexes := make(map[string]int)
	for _, field := range fields {
		groupIndex, ok := groupIndexes[field.Section]
		if !ok {
			groupIndex = len(groups)
			groupIndexes[field.Section] = groupIndex
			groups = append(groups, SetupConfigGroup{Name: field.Section})
		}
		groups[groupIndex].Fields = append(groups[groupIndex].Fields, field)
	}
	return groups
}

type setupConfigFormField struct {
	section string
	label   string
	name    string
	env     string
	secret  bool
	get     func(config.SetupInput) string
	set     func(*config.SetupInput, string)
	options func(string) []SetupConfigOption
}

func (f setupConfigFormField) envManaged() bool {
	return os.Getenv(f.env) != ""
}

func (f setupConfigFormField) optionViews(value string) []SetupConfigOption {
	if f.options == nil {
		return nil
	}
	return f.options(value)
}

var setupConfigFields = []setupConfigFormField{
	{section: "App", label: "App Base URL", name: "setup_app_base_url", env: "APP_BASE_URL", get: func(in config.SetupInput) string { return in.AppBaseURL }, set: func(in *config.SetupInput, v string) { in.AppBaseURL = v }},
	{section: "Media Server", label: "Service", name: "setup_media_server_type", env: "MEDIA_SERVER_TYPE", get: func(in config.SetupInput) string { return in.MediaServerType }, set: func(in *config.SetupInput, v string) { in.MediaServerType = v }, options: mediaServerTypeOptions},
	{section: "Media Server", label: "Internal URL", name: "setup_media_server_url", env: "MEDIA_SERVER_URL", get: func(in config.SetupInput) string { return in.MediaServerURL }, set: func(in *config.SetupInput, v string) { in.MediaServerURL = v }},
	{section: "Media Server", label: "Public URL", name: "setup_media_server_public_url", env: "MEDIA_SERVER_PUBLIC_URL", get: func(in config.SetupInput) string { return in.MediaServerPublicURL }, set: func(in *config.SetupInput, v string) { in.MediaServerPublicURL = v }},
	{section: "Media Server", label: "API Key", name: "setup_media_server_api_key", env: "MEDIA_SERVER_API_KEY", secret: true, get: func(in config.SetupInput) string { return in.MediaServerAPIKey }, set: func(in *config.SetupInput, v string) { in.MediaServerAPIKey = v }},
	{section: "Seerr", label: "Internal URL", name: "setup_seerr_url", env: "SEERR_URL", get: func(in config.SetupInput) string { return in.SeerrURL }, set: func(in *config.SetupInput, v string) { in.SeerrURL = v }},
	{section: "Seerr", label: "Public URL", name: "setup_seerr_public_url", env: "SEERR_PUBLIC_URL", get: func(in config.SetupInput) string { return in.SeerrPublicURL }, set: func(in *config.SetupInput, v string) { in.SeerrPublicURL = v }},
	{section: "Seerr", label: "API Key", name: "setup_seerr_api_key", env: "SEERR_API_KEY", secret: true, get: func(in config.SetupInput) string { return in.SeerrAPIKey }, set: func(in *config.SetupInput, v string) { in.SeerrAPIKey = v }},
	{section: "Sonarr", label: "URL", name: "setup_sonarr_url", env: "SONARR_URL", get: func(in config.SetupInput) string { return in.SonarrURL }, set: func(in *config.SetupInput, v string) { in.SonarrURL = v }},
	{section: "Sonarr", label: "API Key", name: "setup_sonarr_api_key", env: "SONARR_API_KEY", secret: true, get: func(in config.SetupInput) string { return in.SonarrAPIKey }, set: func(in *config.SetupInput, v string) { in.SonarrAPIKey = v }},
	{section: "Radarr", label: "URL", name: "setup_radarr_url", env: "RADARR_URL", get: func(in config.SetupInput) string { return in.RadarrURL }, set: func(in *config.SetupInput, v string) { in.RadarrURL = v }},
	{section: "Radarr", label: "API Key", name: "setup_radarr_api_key", env: "RADARR_API_KEY", secret: true, get: func(in config.SetupInput) string { return in.RadarrAPIKey }, set: func(in *config.SetupInput, v string) { in.RadarrAPIKey = v }},
	{section: "Prowlarr", label: "URL", name: "setup_prowlarr_url", env: "PROWLARR_URL", get: func(in config.SetupInput) string { return in.ProwlarrURL }, set: func(in *config.SetupInput, v string) { in.ProwlarrURL = v }},
	{section: "Prowlarr", label: "API Key", name: "setup_prowlarr_api_key", env: "PROWLARR_API_KEY", secret: true, get: func(in config.SetupInput) string { return in.ProwlarrAPIKey }, set: func(in *config.SetupInput, v string) { in.ProwlarrAPIKey = v }},
}

func mediaServerTypeOptions(value string) []SetupConfigOption {
	selected, _ := config.ParseMediaServerType(value)
	return []SetupConfigOption{
		{Value: config.MediaServerJellyfin.String(), Label: config.MediaServerJellyfin.Label(), Selected: selected == config.MediaServerJellyfin},
		{Value: config.MediaServerEmby.String(), Label: config.MediaServerEmby.Label(), Selected: selected == config.MediaServerEmby},
	}
}
