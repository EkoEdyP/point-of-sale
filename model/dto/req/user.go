package req

// UserUpdateRequest: hanya field yang dikirim yang diubah. Role tidak bisa diubah.
type UserUpdateRequest struct {
	Name     *string `json:"name"`
	Active   *bool   `json:"active"`
	Password *string `json:"password"`
}
