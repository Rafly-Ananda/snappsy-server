package assets

type PresignAssetUploadReq struct {
	Name     string `json:"name" binding:"required"`
	Kind     string `json:"kind" binding:"required"`
	MimeType string `json:"mimeType" binding:"required"`
}

type PresignAssetUploadRes struct {
	UploadUrl   string `json:"uploadUrl"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType"`
}

type CreateAssetReq struct {
	Kind        string `json:"kind" binding:"required"`
	Name        string `json:"name" binding:"required"`
	ObjectKey   string `json:"objectKey" binding:"required"`
	MimeType    string `json:"mimeType" binding:"required"`
	License     string `json:"license"`
	Source      string `json:"source"`
	Attribution string `json:"attribution"`
	Bytes       int64  `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type AssetRes struct {
	ID          string `json:"id"`
	OwnerType   string `json:"ownerType,omitempty"`
	OwnerID     string `json:"ownerId,omitempty"`
	EventID     string `json:"eventId,omitempty"`
	Disabled    bool   `json:"disabled"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	ObjectKey   string `json:"objectKey"`
	Url         string `json:"url"`
	MimeType    string `json:"mimeType"`
	License     string `json:"license"`
	Source      string `json:"source"`
	Attribution string `json:"attribution"`
	Bytes       int64  `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}
