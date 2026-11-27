package notification

type NotificationRequest struct {
	Recipient string
	Message   string
}

type NotificationResult struct {
	Success bool
}
