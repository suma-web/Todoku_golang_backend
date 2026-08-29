package user

import "time"

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Birthday     time.Time `json:"-"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	Bio          string    `json:"bio"`
	Location     string    `json:"location"`
	Website      string    `json:"website"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
}

type LoginResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Birthday  string `json:"birthday"`
	CreatedAt string `json:"created_at"`
	Role      string `json:"role"`
}

type CurrentUserResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	Bio       string `json:"bio"`
	Location  string `json:"location"`
	Website   string `json:"website"`
	CreatedAt string `json:"created_at"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
}

type AdminCreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateProfileRequest struct {
	Name     string `json:"name"`
	Bio      string `json:"bio"`
	Location string `json:"location"`
	Website  string `json:"website"`
}
