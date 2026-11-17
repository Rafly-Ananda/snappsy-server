package drive

type DestinationReq struct {
	FolderID string `json:"folderId" binding:"required"`
}

type SetupLinkRes struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}

type SetupInfoRes struct {
	EventID     string `json:"eventId"`
	EventName   string `json:"eventName"`
	EventSlug   string `json:"eventSlug"`
	Connected   bool   `json:"connected"`
	PickerReady bool   `json:"pickerConfigured"`
}

type SessionRes struct {
	AccessToken string `json:"accessToken"`
	PickerKey   string `json:"pickerApiKey"`
	PickerAppID string `json:"pickerAppId"`
}

type AdminStatusRes struct {
	Configured        bool             `json:"configured"`
	OperatorConnected bool             `json:"operatorConnected"`
	Destination       any              `json:"destination"`
	Counts            map[string]int64 `json:"counts"`
}
