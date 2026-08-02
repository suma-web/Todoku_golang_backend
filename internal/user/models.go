package user

import "time"

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Birthday string `json:"birthday"`
	Password string `json:"password"`
}

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
}

type SignupResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Birthday  string `json:"birthday"`
	CreatedAt string `json:"created_at"`
}

type LoginResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Birthday  string `json:"birthday"`
	CreatedAt string `json:"created_at"`
}

type CurrentUserResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Bio          string `json:"bio"`
	Location     string `json:"location"`
	Website      string `json:"website"`
	CreatedAt    string `json:"created_at"`
	FollowedByMe bool   `json:"followed_by_me"`
}

type FollowResponse struct {
	FollowedByMe bool `json:"followed_by_me"`
}

type UpdateProfileRequest struct {
	Name     string `json:"name"`
	Bio      string `json:"bio"`
	Location string `json:"location"`
	Website  string `json:"website"`
}
