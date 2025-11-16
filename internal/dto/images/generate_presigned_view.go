package images

type GeneratePresignedUrlView struct {
	Id  	 string `json:"id"`
	Url      string `json:"url"`
	Captions string `json:"captions"`
	From     string `json:"from"`
	Orientation string `json:"orientation"`
	CreatedAt string  `json:"createdAt"`
}
