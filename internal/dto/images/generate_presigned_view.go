package images

type GeneratePresignedUrlView struct {
	Url      string `json:"url"`
	Captions string `json:"captions"`
	From     string `json:"from"`
}
