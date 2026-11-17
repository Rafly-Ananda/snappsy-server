package admin

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRes struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
}

type MeRes struct {
	Username string `json:"username"`
}
