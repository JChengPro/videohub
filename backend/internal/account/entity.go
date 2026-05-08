package account

type Account struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Username string `gorm:"unique" json:"username"`
	Password string `json:"-"`
	//Token 的作用是：保存当前有效 token，让旧 token 可以失效。
	Token string `json:"-"`
}

// RegisterRequest 用来接收前端传来的注册 JSON
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type FindByIDRequest struct {
	ID uint `json:"id"`
}

type FindByUsernameRequest struct {
	Username string `json:"username"`
}

type RenameRequest struct {
	NewUsername string `json:"new_username"`
}
