package account

type Account struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	AccountName     string `gorm:"column:account_name;size:24;not null;uniqueIndex:idx_accounts_account_name" json:"account_name"`
	Username        string `gorm:"size:24;not null" json:"username"`
	Password        string `json:"-"`
	AvatarObjectKey string `gorm:"type:varchar(500);not null;default:''" json:"-"`
	LegacyLogin     bool   `gorm:"column:legacy_login;not null;default:false" json:"-"`
	//Token 的作用是：保存当前有效 token，让旧 token 可以失效。
	Token string `json:"-"`
}

// RegisterRequest 用来接收前端传来的注册 JSON
type RegisterRequest struct {
	AccountName string `json:"account_name"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

type LoginRequest struct {
	AccountName string `json:"account_name"`
	// Username 仅用于兼容升级前的客户端，新的客户端统一提交 account_name。
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type FindByIDRequest struct {
	ID uint `json:"id"`
}

type FindByUsernameRequest struct {
	Username string `json:"username"`
}

type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type SearchResponse struct {
	Users      []Account `json:"users"`
	HasMore    bool      `json:"has_more"`
	NextOffset int       `json:"next_offset"`
}

type RenameRequest struct {
	NewUsername string `json:"new_username"`
}

type CheckAccountNameRequest struct {
	AccountName string `json:"account_name"`
}

type CheckAccountNameResponse struct {
	AccountName string `json:"account_name"`
	Available   bool   `json:"available"`
}

type RegisterResponse struct {
	Message     string `json:"message"`
	AccountName string `json:"account_name"`
	Username    string `json:"username"`
}

type AvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}
