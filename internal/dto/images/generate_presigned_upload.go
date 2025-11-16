package images

type GeneratePresignedUrlReq struct {
	Username string `form:"username" json:"username" binding:"required"`
	EventId  string `form:"eventId" json:"eventId" binding:"required"`
	MimeType string `form:"mimeType" json:"mimeType" binding:"required"`
}

type GeneratePresignedUrlRes struct {
	UploadUrl   string
	ObjectKey   string
	ContentType string
}
