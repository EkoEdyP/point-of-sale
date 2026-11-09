package res

type UserResponse struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Active   bool   `json:"active"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"` // detik
	User        UserResponse `json:"user"`
}
