package images

type GeneratePresignedUrlReq struct {
	Username string `form:"username" json:"username" binding:"required"`
	PhotoURL string `form:"photoUrl" json:"photoUrl" binding:"required"`
	EventId  string `form:"eventId" json:"eventId" binding:"required"`
}

type GeneratePresignedUrlRes struct {
	UploadUrl   string
	ObjectKey   string
	ContentType string
}
