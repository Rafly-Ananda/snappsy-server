package events

type CreateEventReq struct {
	EventName   string `json:"eventName" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type CreateEventRes struct {
	ID string `json:"id"`
}
