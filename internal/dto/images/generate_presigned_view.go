package images

type GeneratePresignedUrlView struct {
	Id          string `json:"id"`
	Kind        string `json:"kind"`
	Url         string `json:"url"`
	PlaybackUrl string `json:"playbackUrl,omitempty"`
	Captions    string `json:"captions"`
	From        string `json:"from"`
	Orientation string `json:"orientation"`
	CreatedAt   string `json:"createdAt"`
}
