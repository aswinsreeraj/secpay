package response

// ErrorResponse represents standard error payloads.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents standard success text/message payloads.
type SuccessResponse struct {
	Message string `json:"message"`
}

// RegisterResponse represents registration endpoint returns.
type RegisterResponse struct {
	Message string      `json:"message"`
	User    UserSummary `json:"user"`
}

// UserSummary defines limited User details returned for registration responses.
type UserSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	KYCStatus string `json:"kyc_status"`
}

// LoginResponse represents login endpoint returns.
type LoginResponse struct {
	Token string `json:"token"`
}

// MFAVerifyResponse represents MFA verify endpoint returns.
type MFAVerifyResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// KYCVerifyResponse represents KYC verify endpoint returns.
type KYCVerifyResponse struct {
	Message   string `json:"message"`
	KYCStatus string `json:"kyc_status"`
}

// PaymentAcceptedResponse represents accepted asynchronous payment enqueues.
type PaymentAcceptedResponse struct {
	Message string `json:"message"`
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
}
