package req

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,max=100"`
	Username string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	Role     string `json:"role" validate:"required,oneof=admin cashier"`
}
